package httpapi

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/cookies-bnb/platform/services/api/internal/auth"
	"github.com/gin-gonic/gin"
)

const (
	sessionCookieName  = "bnb_session"
	csrfCookieName     = "bnb_csrf"
	identityContextKey = "authenticated_identity"
)

type AuthHandler struct {
	service      *auth.Service
	secureCookie bool
}

func NewAuthHandler(service *auth.Service, secureCookie bool) *AuthHandler {
	return &AuthHandler{service: service, secureCookie: secureCookie}
}

func (h *AuthHandler) RegisterRoutes(api *gin.RouterGroup) {
	api.POST("/auth/register", h.register)
	api.POST("/auth/login", h.login)
	api.GET("/auth/me", h.requireSession(), h.me)
	api.POST("/auth/logout", h.requireSession(), h.logout)
}

type registerRequest struct {
	Email        string `json:"email"`
	FullName     string `json:"full_name"`
	Password     string `json:"password"`
	BusinessName string `json:"business_name"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *AuthHandler) register(c *gin.Context) {
	var request registerRequest
	if err := decodeRequest(c, &request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_request"}})
		return
	}

	session, err := h.service.Register(c.Request.Context(), request.Email, request.FullName, request.Password, request.BusinessName)
	if err != nil {
		writeAuthError(c, err)
		return
	}
	h.setSessionCookies(c, session)
	c.JSON(http.StatusCreated, gin.H{"user": session.User})
}

func (h *AuthHandler) login(c *gin.Context) {
	var request loginRequest
	if err := decodeRequest(c, &request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_request"}})
		return
	}

	session, err := h.service.Login(c.Request.Context(), request.Email, request.Password)
	if err != nil {
		writeAuthError(c, err)
		return
	}
	h.setSessionCookies(c, session)
	c.JSON(http.StatusOK, gin.H{"user": session.User})
}

func (h *AuthHandler) requireSession() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie(sessionCookieName)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "unauthenticated"}})
			return
		}
		identity, err := h.service.Authenticate(c.Request.Context(), token)
		if err != nil {
			if errors.Is(err, auth.ErrUnauthenticated) {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "unauthenticated"}})
				return
			}
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "internal_error"}})
			return
		}
		c.Set(identityContextKey, identity)
		c.Next()
	}
}

func (h *AuthHandler) me(c *gin.Context) {
	identity := c.MustGet(identityContextKey).(auth.Identity)
	c.JSON(http.StatusOK, gin.H{"user": identity.User})
}

func (h *AuthHandler) logout(c *gin.Context) {
	identity := c.MustGet(identityContextKey).(auth.Identity)
	csrfHeader := c.GetHeader("X-CSRF-Token")
	if !csrfHeaderMatchesCookie(c) || !h.service.VerifyCSRF(identity, csrfHeader) {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": gin.H{"code": "csrf_forbidden"}})
		return
	}

	token, _ := c.Cookie(sessionCookieName)
	if err := h.service.Logout(c.Request.Context(), token); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "internal_error"}})
		return
	}
	h.clearSessionCookies(c)
	c.Status(http.StatusNoContent)
}

func (h *AuthHandler) setSessionCookies(c *gin.Context, session auth.Session) {
	maxAge := int(time.Until(session.ExpiresAt).Seconds())
	http.SetCookie(c.Writer, &http.Cookie{
		Name: sessionCookieName, Value: session.Token, Path: "/api/v1/auth",
		Expires: session.ExpiresAt, MaxAge: maxAge, HttpOnly: true,
		Secure: h.secureCookie, SameSite: http.SameSiteStrictMode,
	})
	http.SetCookie(c.Writer, &http.Cookie{
		Name: csrfCookieName, Value: session.CSRFToken, Path: "/",
		Expires: session.ExpiresAt, MaxAge: maxAge, HttpOnly: false,
		Secure: h.secureCookie, SameSite: http.SameSiteStrictMode,
	})
}

func (h *AuthHandler) clearSessionCookies(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name: sessionCookieName, Value: "", Path: "/api/v1/auth", MaxAge: -1,
		HttpOnly: true, Secure: h.secureCookie, SameSite: http.SameSiteStrictMode,
	})
	http.SetCookie(c.Writer, &http.Cookie{
		Name: csrfCookieName, Value: "", Path: "/", MaxAge: -1,
		Secure: h.secureCookie, SameSite: http.SameSiteStrictMode,
	})
}

func decodeRequest(c *gin.Context, target any) error {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request must contain one JSON value")
	}
	return nil
}

func writeAuthError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, auth.ErrInvalidInput):
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_input"}})
	case errors.Is(err, auth.ErrEmailTaken):
		c.JSON(http.StatusConflict, gin.H{"error": gin.H{"code": "email_unavailable"}})
	case errors.Is(err, auth.ErrInvalidCredentials), errors.Is(err, auth.ErrUnauthenticated):
		c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "invalid_credentials"}})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "internal_error"}})
	}
}

func csrfHeaderMatchesCookie(c *gin.Context) bool {
	header := c.GetHeader("X-CSRF-Token")
	cookie, err := c.Cookie(csrfCookieName)
	return err == nil && header != "" && subtle.ConstantTimeCompare([]byte(header), []byte(cookie)) == 1
}
