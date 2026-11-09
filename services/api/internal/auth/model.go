package auth

import (
	"context"
	"time"
)

type Membership struct {
	BusinessID   string `json:"business_id"`
	BusinessName string `json:"business_name"`
	Role         string `json:"role"`
}

type User struct {
	ID              string       `json:"id"`
	Email           string       `json:"email"`
	FullName        string       `json:"full_name"`
	IsPlatformAdmin bool         `json:"is_platform_admin"`
	Memberships     []Membership `json:"memberships"`
}

type Identity struct {
	User      User
	CSRFHash  []byte
	ExpiresAt time.Time
}

type Session struct {
	Token     string
	CSRFToken string
	ExpiresAt time.Time
	User      User
}

type Repository interface {
	CreateAccount(ctx context.Context, email, fullName, passwordHash, businessName string, tokenHash, csrfHash []byte, expiresAt time.Time) (User, error)
	FindUserByEmail(ctx context.Context, email string) (User, string, error)
	CreateSession(ctx context.Context, userID string, tokenHash, csrfHash []byte, expiresAt time.Time) error
	FindSession(ctx context.Context, tokenHash []byte) (Identity, error)
	RevokeSession(ctx context.Context, tokenHash []byte) error
}

type Authenticator interface {
	Authenticate(ctx context.Context, token string) (Identity, error)
	Logout(ctx context.Context, token string) error
}
