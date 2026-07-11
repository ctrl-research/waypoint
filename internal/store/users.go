package store

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ctrl-research/waypoint/internal/store/sqlcgen"
)

type (
	User             = sqlcgen.User
	CreateUserParams = sqlcgen.CreateUserParams
)

type Users struct {
	q *sqlcgen.Queries
}

func NewUsers(pool *pgxpool.Pool) *Users {
	return &Users{q: sqlcgen.New(pool)}
}

func (s *Users) Create(ctx context.Context, p CreateUserParams) (User, error) {
	u, err := s.q.CreateUser(ctx, p)
	return u, translate(err)
}

func (s *Users) ByID(ctx context.Context, id uuid.UUID) (User, error) {
	u, err := s.q.UserByID(ctx, id)
	return u, translate(err)
}

func (s *Users) ByEmail(ctx context.Context, email string) (User, error) {
	u, err := s.q.UserByEmail(ctx, email)
	return u, translate(err)
}

func (s *Users) ByGoogleSub(ctx context.Context, sub string) (User, error) {
	u, err := s.q.UserByGoogleSub(ctx, &sub)
	return u, translate(err)
}

// UpdateProfile refreshes the fields Google reports on each sign-in.
func (s *Users) UpdateProfile(ctx context.Context, id uuid.UUID, displayName string, avatarURL *string) (User, error) {
	u, err := s.q.UpdateUserProfile(ctx, sqlcgen.UpdateUserProfileParams{
		ID: id, DisplayName: displayName, AvatarURL: avatarURL,
	})
	return u, translate(err)
}

// LinkGoogle attaches a Google identity to an existing account (matched by
// email at sign-in) and refreshes the profile fields Google reports.
func (s *Users) LinkGoogle(ctx context.Context, id uuid.UUID, googleSub string, displayName string, avatarURL *string) (User, error) {
	u, err := s.q.LinkGoogle(ctx, sqlcgen.LinkGoogleParams{
		ID: id, GoogleSub: &googleSub, DisplayName: displayName, AvatarURL: avatarURL,
	})
	return u, translate(err)
}

// SetPassword replaces the user's password hash (used by the seed command).
func (s *Users) SetPassword(ctx context.Context, id uuid.UUID, passwordHash string) (User, error) {
	u, err := s.q.SetPassword(ctx, sqlcgen.SetPasswordParams{ID: id, PasswordHash: &passwordHash})
	return u, translate(err)
}

func (s *Users) Count(ctx context.Context) (int64, error) {
	return s.q.CountUsers(ctx)
}
