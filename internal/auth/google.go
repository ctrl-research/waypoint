package auth

import (
	"context"
	"errors"

	"github.com/ctrl-research/waypoint/internal/store"
)

// issuerOverride lets tests point discovery at a fake IdP; empty in production.
var issuerOverride = ""

func newGoogleProvider(ctx context.Context, opts Options) (*ssoProvider, error) {
	issuer := "https://accounts.google.com"
	if issuerOverride != "" {
		issuer = issuerOverride
	}
	// Google always sets email_verified; require it.
	return newSSOProvider(ctx, "google", "Google", issuer, opts.GoogleClientID, opts.GoogleClientSecret, opts.BaseURL, true)
}

// ErrEmailNotAllowed is returned when a new sign-up is not on the allowlist.
var ErrEmailNotAllowed = errors.New("email not allowed to sign up")

// upsertGoogleUser finds the user by Google subject, links an existing
// account with the same email, or creates a new user. The first user on the
// instance becomes admin; after that, new sign-ups must be on the
// WAYPOINT_ALLOWED_EMAILS allowlist (existing users always sign in).
func (s *Service) upsertGoogleUser(ctx context.Context, sub, email, name string, picture string) (store.User, error) {
	var avatar *string
	if picture != "" {
		avatar = &picture
	}

	user, err := s.users.ByGoogleSub(ctx, sub)
	switch {
	case err == nil:
		return s.users.UpdateProfile(ctx, user.ID, name, avatar)
	case !errors.Is(err, store.ErrNotFound):
		return store.User{}, err
	}

	if user, err = s.users.ByEmail(ctx, email); err == nil {
		return s.users.LinkGoogle(ctx, user.ID, sub, name, avatar)
	} else if !errors.Is(err, store.ErrNotFound) {
		return store.User{}, err
	}

	count, err := s.users.Count(ctx)
	if err != nil {
		return store.User{}, err
	}
	if count > 0 && !s.emailAllowed(email) {
		return store.User{}, ErrEmailNotAllowed
	}
	return s.users.Create(ctx, store.CreateUserParams{
		Email:       email,
		DisplayName: name,
		AvatarURL:   avatar,
		GoogleSub:   &sub,
		IsAdmin:     count == 0,
	})
}
