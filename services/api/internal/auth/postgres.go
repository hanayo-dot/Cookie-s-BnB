package auth

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct {
	database *pgxpool.Pool
}

func NewPostgresStore(database *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{database: database}
}

func (s *PostgresStore) CreateAccount(ctx context.Context, email, fullName, passwordHash, businessName string, tokenHash, csrfHash []byte, expiresAt time.Time) (User, error) {
	tx, err := s.database.Begin(ctx)
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback(ctx)

	var user User
	err = tx.QueryRow(ctx, `
		INSERT INTO users (email, full_name, password_hash)
		VALUES ($1, $2, $3)
		RETURNING id::text, email, full_name, is_platform_admin`, email, fullName, passwordHash,
	).Scan(&user.ID, &user.Email, &user.FullName, &user.IsPlatformAdmin)
	if err != nil {
		var postgresError *pgconn.PgError
		if errors.As(err, &postgresError) && postgresError.Code == "23505" && postgresError.ConstraintName == "users_email_unique" {
			return User{}, ErrEmailTaken
		}
		return User{}, err
	}

	var businessID string
	if err := tx.QueryRow(ctx, `INSERT INTO businesses (name) VALUES ($1) RETURNING id::text`, businessName).Scan(&businessID); err != nil {
		return User{}, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO business_memberships (business_id, user_id, role, status)
		VALUES ($1, $2, 'business_owner', 'active')`, businessID, user.ID,
	); err != nil {
		return User{}, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO auth_sessions (token_hash, csrf_hash, user_id, expires_at)
		VALUES ($1, $2, $3, $4)`, tokenHash, csrfHash, user.ID, expiresAt,
	); err != nil {
		return User{}, err
	}

	user.Memberships, err = loadMemberships(ctx, tx, user.ID)
	if err != nil {
		return User{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return User{}, err
	}
	return user, nil
}

func (s *PostgresStore) FindUserByEmail(ctx context.Context, email string) (User, string, error) {
	var user User
	var passwordHash string
	err := s.database.QueryRow(ctx, `
		SELECT id::text, email, full_name, is_platform_admin, password_hash
		FROM users WHERE email = $1 AND deleted_at IS NULL`, email,
	).Scan(&user.ID, &user.Email, &user.FullName, &user.IsPlatformAdmin, &passwordHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, "", ErrInvalidCredentials
	}
	if err != nil {
		return User{}, "", err
	}
	user.Memberships, err = loadMemberships(ctx, s.database, user.ID)
	if err != nil {
		return User{}, "", err
	}
	return user, passwordHash, nil
}

func (s *PostgresStore) CreateSession(ctx context.Context, userID string, tokenHash, csrfHash []byte, expiresAt time.Time) error {
	_, err := s.database.Exec(ctx, `
		INSERT INTO auth_sessions (token_hash, csrf_hash, user_id, expires_at)
		VALUES ($1, $2, $3, $4)`, tokenHash, csrfHash, userID, expiresAt,
	)
	return err
}

func (s *PostgresStore) FindSession(ctx context.Context, tokenHash []byte) (Identity, error) {
	var identity Identity
	err := s.database.QueryRow(ctx, `
		SELECT u.id::text, u.email, u.full_name, u.is_platform_admin, s.csrf_hash, s.expires_at
		FROM auth_sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = $1 AND s.revoked_at IS NULL AND s.expires_at > now() AND u.deleted_at IS NULL`, tokenHash,
	).Scan(&identity.User.ID, &identity.User.Email, &identity.User.FullName, &identity.User.IsPlatformAdmin, &identity.CSRFHash, &identity.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Identity{}, ErrUnauthenticated
	}
	if err != nil {
		return Identity{}, err
	}
	identity.User.Memberships, err = loadMemberships(ctx, s.database, identity.User.ID)
	if err != nil {
		return Identity{}, err
	}
	return identity, nil
}

func (s *PostgresStore) RevokeSession(ctx context.Context, tokenHash []byte) error {
	_, err := s.database.Exec(ctx, `
		UPDATE auth_sessions SET revoked_at = now()
		WHERE token_hash = $1 AND revoked_at IS NULL`, tokenHash,
	)
	return err
}

type queryRower interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func loadMemberships(ctx context.Context, queryer queryRower, userID string) ([]Membership, error) {
	rows, err := queryer.Query(ctx, `
		SELECT b.id::text, b.name, m.role::text
		FROM business_memberships m
		JOIN businesses b ON b.id = m.business_id
	WHERE m.user_id = $1 AND m.status = 'active' AND b.deleted_at IS NULL
		ORDER BY b.name`, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	memberships := make([]Membership, 0)
	for rows.Next() {
		var membership Membership
		if err := rows.Scan(&membership.BusinessID, &membership.BusinessName, &membership.Role); err != nil {
			return nil, err
		}
		memberships = append(memberships, membership)
	}
	return memberships, rows.Err()
}
