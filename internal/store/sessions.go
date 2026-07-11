package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Sessions stores server-side sessions keyed by the SHA-256 hash of the
// session token. The raw token only ever lives in the user's cookie;
// generating and hashing it is the auth package's job.
type Sessions struct {
	pool *pgxpool.Pool
}

func NewSessions(pool *pgxpool.Pool) *Sessions {
	return &Sessions{pool: pool}
}

func (s *Sessions) Create(ctx context.Context, tokenHash []byte, userID uuid.UUID, expiresAt time.Time) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO sessions (token_hash, user_id, expires_at)
		VALUES ($1, $2, $3)`,
		tokenHash, userID, expiresAt)
	return err
}

// UserByToken resolves an unexpired session to its user and bumps
// last_seen_at. Returns ErrNotFound for unknown or expired tokens.
func (s *Sessions) UserByToken(ctx context.Context, tokenHash []byte) (User, error) {
	return scanUser(s.pool.QueryRow(ctx, `
		UPDATE sessions SET last_seen_at = now()
		FROM users
		WHERE sessions.token_hash = $1
		  AND sessions.expires_at > now()
		  AND users.id = sessions.user_id
		RETURNING users.id, users.email, users.display_name, users.avatar_url,
		          users.google_sub, users.password_hash, users.is_admin, users.created_at`,
		tokenHash))
}

func (s *Sessions) Delete(ctx context.Context, tokenHash []byte) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE token_hash = $1`, tokenHash)
	return err
}

func (s *Sessions) DeleteForUser(ctx context.Context, userID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE user_id = $1`, userID)
	return err
}

// DeleteExpired removes expired sessions and reports how many were deleted.
// Intended to be called periodically from the server.
func (s *Sessions) DeleteExpired(ctx context.Context) (int64, error) {
	tag, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE expires_at <= now()`)
	return tag.RowsAffected(), err
}
