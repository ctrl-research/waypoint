package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ctrl-research/waypoint/internal/store/sqlcgen"
)

// Sessions stores server-side sessions keyed by the SHA-256 hash of the
// session token. The raw token only ever lives in the user's cookie;
// generating and hashing it is the auth package's job.
type Sessions struct {
	q *sqlcgen.Queries
}

func NewSessions(pool *pgxpool.Pool) *Sessions {
	return &Sessions{q: sqlcgen.New(pool)}
}

func (s *Sessions) Create(ctx context.Context, tokenHash []byte, userID uuid.UUID, expiresAt time.Time) error {
	return s.q.CreateSession(ctx, sqlcgen.CreateSessionParams{
		TokenHash: tokenHash, UserID: userID, ExpiresAt: expiresAt,
	})
}

// UserByToken resolves an unexpired session to its user and bumps
// last_seen_at. Returns ErrNotFound for unknown or expired tokens.
func (s *Sessions) UserByToken(ctx context.Context, tokenHash []byte) (User, error) {
	u, err := s.q.SessionUserByToken(ctx, tokenHash)
	return u, translate(err)
}

func (s *Sessions) Delete(ctx context.Context, tokenHash []byte) error {
	return s.q.DeleteSession(ctx, tokenHash)
}

func (s *Sessions) DeleteForUser(ctx context.Context, userID uuid.UUID) error {
	return s.q.DeleteSessionsForUser(ctx, userID)
}

// DeleteExpired removes expired sessions and reports how many were deleted.
// Intended to be called periodically from the server.
func (s *Sessions) DeleteExpired(ctx context.Context) (int64, error) {
	return s.q.DeleteExpiredSessions(ctx)
}
