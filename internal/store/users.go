package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type User struct {
	ID           uuid.UUID
	Email        string
	DisplayName  string
	AvatarURL    *string
	GoogleSub    *string
	PasswordHash *string
	IsAdmin      bool
	CreatedAt    time.Time
}

type CreateUserParams struct {
	Email        string
	DisplayName  string
	AvatarURL    *string
	GoogleSub    *string
	PasswordHash *string
	IsAdmin      bool
}

type Users struct {
	pool *pgxpool.Pool
}

func NewUsers(pool *pgxpool.Pool) *Users {
	return &Users{pool: pool}
}

const userColumns = `id, email, display_name, avatar_url, google_sub, password_hash, is_admin, created_at`

func scanUser(row pgx.Row) (User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Email, &u.DisplayName, &u.AvatarURL, &u.GoogleSub, &u.PasswordHash, &u.IsAdmin, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}

func (s *Users) Create(ctx context.Context, p CreateUserParams) (User, error) {
	return scanUser(s.pool.QueryRow(ctx, `
		INSERT INTO users (email, display_name, avatar_url, google_sub, password_hash, is_admin)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING `+userColumns,
		p.Email, p.DisplayName, p.AvatarURL, p.GoogleSub, p.PasswordHash, p.IsAdmin))
}

func (s *Users) ByID(ctx context.Context, id uuid.UUID) (User, error) {
	return scanUser(s.pool.QueryRow(ctx,
		`SELECT `+userColumns+` FROM users WHERE id = $1`, id))
}

func (s *Users) ByEmail(ctx context.Context, email string) (User, error) {
	return scanUser(s.pool.QueryRow(ctx,
		`SELECT `+userColumns+` FROM users WHERE email = $1`, email))
}

func (s *Users) ByGoogleSub(ctx context.Context, sub string) (User, error) {
	return scanUser(s.pool.QueryRow(ctx,
		`SELECT `+userColumns+` FROM users WHERE google_sub = $1`, sub))
}

// UpdateProfile refreshes the fields Google reports on each sign-in.
func (s *Users) UpdateProfile(ctx context.Context, id uuid.UUID, displayName string, avatarURL *string) (User, error) {
	return scanUser(s.pool.QueryRow(ctx, `
		UPDATE users SET display_name = $2, avatar_url = $3 WHERE id = $1
		RETURNING `+userColumns,
		id, displayName, avatarURL))
}

// LinkGoogle attaches a Google identity to an existing account (matched by
// email at sign-in) and refreshes the profile fields Google reports.
func (s *Users) LinkGoogle(ctx context.Context, id uuid.UUID, googleSub string, displayName string, avatarURL *string) (User, error) {
	return scanUser(s.pool.QueryRow(ctx, `
		UPDATE users SET google_sub = $2, display_name = $3, avatar_url = $4
		WHERE id = $1
		RETURNING `+userColumns,
		id, googleSub, displayName, avatarURL))
}

// SetPassword replaces the user's password hash (used by the seed command).
func (s *Users) SetPassword(ctx context.Context, id uuid.UUID, passwordHash string) (User, error) {
	return scanUser(s.pool.QueryRow(ctx, `
		UPDATE users SET password_hash = $2 WHERE id = $1
		RETURNING `+userColumns,
		id, passwordHash))
}

func (s *Users) Count(ctx context.Context) (int64, error) {
	var n int64
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&n)
	return n, err
}
