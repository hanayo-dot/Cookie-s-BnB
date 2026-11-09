package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidInput       = errors.New("invalid input")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrEmailTaken         = errors.New("email already registered")
	ErrUnauthenticated    = errors.New("unauthenticated")
)

const sessionLifetime = 12 * time.Hour

type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) Register(ctx context.Context, email, fullName, password, businessName string) (Session, error) {
	email, err := normalizeEmail(email)
	if err != nil {
		return Session{}, err
	}
	fullName = strings.TrimSpace(fullName)
	businessName = strings.TrimSpace(businessName)
	if !validText(fullName, 1, 120) || !validText(businessName, 2, 120) || len(password) < 12 || len(password) > 72 {
		return Session{}, ErrInvalidInput
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return Session{}, err
	}
	token, tokenHash, err := randomToken()
	if err != nil {
		return Session{}, err
	}
	csrfToken, csrfHash, err := randomToken()
	if err != nil {
		return Session{}, err
	}
	expiresAt := time.Now().Add(sessionLifetime)
	user, err := s.repository.CreateAccount(ctx, email, fullName, string(passwordHash), businessName, tokenHash, csrfHash, expiresAt)
	if err != nil {
		return Session{}, err
	}
	return Session{Token: token, CSRFToken: csrfToken, ExpiresAt: expiresAt, User: user}, nil
}

func (s *Service) Login(ctx context.Context, email, password string) (Session, error) {
	email, err := normalizeEmail(email)
	if err != nil {
		return Session{}, ErrInvalidCredentials
	}
	user, passwordHash, err := s.repository.FindUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			return Session{}, ErrInvalidCredentials
		}
		return Session{}, err
	}
	if bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)) != nil {
		return Session{}, ErrInvalidCredentials
	}

	token, tokenHash, err := randomToken()
	if err != nil {
		return Session{}, err
	}
	csrfToken, csrfHash, err := randomToken()
	if err != nil {
		return Session{}, err
	}
	expiresAt := time.Now().Add(sessionLifetime)
	if err := s.repository.CreateSession(ctx, user.ID, tokenHash, csrfHash, expiresAt); err != nil {
		return Session{}, err
	}
	return Session{Token: token, CSRFToken: csrfToken, ExpiresAt: expiresAt, User: user}, nil
}

func (s *Service) Authenticate(ctx context.Context, token string) (Identity, error) {
	tokenHash, err := hashToken(token)
	if err != nil {
		return Identity{}, ErrUnauthenticated
	}
	identity, err := s.repository.FindSession(ctx, tokenHash)
	if err != nil {
		return Identity{}, ErrUnauthenticated
	}
	return identity, nil
}

func (s *Service) Logout(ctx context.Context, token string) error {
	tokenHash, err := hashToken(token)
	if err != nil {
		return ErrUnauthenticated
	}
	return s.repository.RevokeSession(ctx, tokenHash)
}

func (s *Service) VerifyCSRF(identity Identity, token string) bool {
	hash, err := hashToken(token)
	return err == nil && subtle.ConstantTimeCompare(identity.CSRFHash, hash) == 1
}

func normalizeEmail(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address != email || !validText(email, 3, 254) {
		return "", ErrInvalidInput
	}
	return email, nil
}

func validText(value string, min, max int) bool {
	length := utf8.RuneCountInString(value)
	return length >= min && length <= max
}

func randomToken() (string, []byte, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, err
	}
	hash := sha256.Sum256(raw)
	return base64.RawURLEncoding.EncodeToString(raw), hash[:], nil
}

func hashToken(token string) ([]byte, error) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(raw) != 32 {
		return nil, ErrUnauthenticated
	}
	hash := sha256.Sum256(raw)
	return hash[:], nil
}
