package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/cookies-bnb/platform/services/api/internal/auth"
	"github.com/gin-gonic/gin"
)

type DatabasePinger func(context.Context) error

func NewRouter(pingDatabase DatabasePinger, authHandler *AuthHandler, allowedOrigin string) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())

	api := router.Group("/api/v1")
	api.Use(originGuard(allowedOrigin))
	api.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	api.GET("/readyz", func(c *gin.Context) {
		if pingDatabase == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready"})
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := pingDatabase(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})
	if authHandler != nil {
		authHandler.RegisterRoutes(api)
	}

	return router
}

func originGuard(allowedOrigin string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead && c.Request.Method != http.MethodOptions {
			if allowedOrigin == "" || c.GetHeader("Origin") != allowedOrigin {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": gin.H{"code": "origin_forbidden"}})
				return
			}
		}
		c.Next()
	}
}

var _ auth.Authenticator = (*auth.Service)(nil)
