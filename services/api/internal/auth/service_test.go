package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type memoryRepository struct {
	user           User
	passwordHash   string
	tokenHash      []byte
	csrfHash       []byte
	expiresAt      time.Time
	sessionCreated bool
}

func (r *memoryRepository) CreateAccount(_ context.Context, email, fullName, passwordHash, businessName string, tokenHash, csrfHash []byte, expiresAt time.Time) (User, error) {
	if r.user.ID != "" {
		return User{}, ErrEmailTaken
	}
	r.user = User{
		ID:       "user-1",
		Email:    email,
		FullName: fullName,
		Memberships: []Membership{{
			BusinessID: "business-1", BusinessName: businessName, Role: "business_owner",
		}},
	}
	r.passwordHash = passwordHash
	r.saveSession(tokenHash, csrfHash, expiresAt)
	return r.user, nil
}

func (r *memoryRepository) FindUserByEmail(_ context.Context, email string) (User, string, error) {
	if r.user.ID == "" || r.user.Email != email {
		return User{}, "", ErrInvalidCredentials
	}
	return r.user, r.passwordHash, nil
}

func (r *memoryRepository) CreateSession(_ context.Context, _ string, tokenHash, csrfHash []byte, expiresAt time.Time) error {
	r.saveSession(tokenHash, csrfHash, expiresAt)
	return nil
}

func (r *memoryRepository) FindSession(_ context.Context, tokenHash []byte) (Identity, error) {
	if string(r.tokenHash) != string(tokenHash) || !r.expiresAt.After(time.Now()) {
		return Identity{}, ErrUnauthenticated
	}
	return Identity{User: r.user, CSRFHash: r.csrfHash, ExpiresAt: r.expiresAt}, nil
}

func (r *memoryRepository) RevokeSession(_ context.Context, tokenHash []byte) error {
	if string(r.tokenHash) != string(tokenHash) {
		return ErrUnauthenticated
	}
	r.expiresAt = time.Time{}
	return nil
}

func (r *memoryRepository) saveSession(tokenHash, csrfHash []byte, expiresAt time.Time) {
	r.tokenHash = append([]byte(nil), tokenHash...)
	r.csrfHash = append([]byte(nil), csrfHash...)
	r.expiresAt = expiresAt
	r.sessionCreated = true
}

func TestRegisterCreatesBusinessOwnerAndHashedSession(t *testing.T) {
	repository := &memoryRepository{}
	service := NewService(repository)

	session, err := service.Register(context.Background(), " Owner@Example.com ", "  Owner Name  ", "correct horse battery", "  Coast Stays  ")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if session.User.Email != "owner@example.com" || session.User.FullName != "Owner Name" {
		t.Fatalf("unexpected user: %+v", session.User)
	}
	if len(session.User.Memberships) != 1 || session.User.Memberships[0].Role != "business_owner" {
		t.Fatalf("owner membership missing: %+v", session.User.Memberships)
	}
	if bcrypt.CompareHashAndPassword([]byte(repository.passwordHash), []byte("correct horse battery")) != nil {
		t.Fatal("password was not stored as a valid bcrypt hash")
	}
	if session.Token == "" || session.CSRFToken == "" || !repository.sessionCreated {
		t.Fatal("session credentials were not created")
	}
	if len(repository.tokenHash) != 32 || len(repository.csrfHash) != 32 {
		t.Fatal("session secrets were not stored as SHA-256 hashes")
	}
	identity, err := service.Authenticate(context.Background(), session.Token)
	if err != nil {
		t.Fatalf("authenticate new session: %v", err)
	}
	if !service.VerifyCSRF(identity, session.CSRFToken) || service.VerifyCSRF(identity, "invalid-token") {
		t.Fatal("CSRF token verification did not match session")
	}
	if err := service.Logout(context.Background(), session.Token); err != nil {
		t.Fatalf("logout: %v", err)
	}
	if _, err := service.Authenticate(context.Background(), session.Token); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("got error %v after logout, want unauthenticated", err)
	}
}

func TestLoginRejectsInvalidCredentials(t *testing.T) {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("correct horse battery"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	repository := &memoryRepository{
		user:         User{ID: "user-1", Email: "owner@example.com"},
		passwordHash: string(passwordHash),
	}
	service := NewService(repository)

	if _, err := service.Login(context.Background(), "owner@example.com", "wrong password"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("got error %v, want invalid credentials", err)
	}
	if repository.sessionCreated {
		t.Fatal("invalid credentials created a session")
	}
}

func TestRegisterRejectsShortPassword(t *testing.T) {
	if _, err := NewService(&memoryRepository{}).Register(context.Background(), "owner@example.com", "Owner", "short", "Coast Stays"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("got error %v, want invalid input", err)
	}
}
