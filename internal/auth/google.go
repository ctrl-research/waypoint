package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/ctrl-research/waypoint/internal/store"
)

const (
	stateCookie    = "waypoint_oauth_state"
	verifierCookie = "waypoint_oauth_verifier"
)

type googleProvider struct {
	oauth    oauth2.Config
	verifier *oidc.IDTokenVerifier
}

// issuerOverride lets tests point discovery at a fake IdP; empty in production.
var issuerOverride = ""

func newGoogleProvider(ctx context.Context, opts Options) (*googleProvider, error) {
	issuer := "https://accounts.google.com"
	if issuerOverride != "" {
		issuer = issuerOverride
	}
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, fmt.Errorf("google oidc discovery: %w", err)
	}
	return &googleProvider{
		oauth: oauth2.Config{
			ClientID:     opts.GoogleClientID,
			ClientSecret: opts.GoogleClientSecret,
			Endpoint:     provider.Endpoint(),
			RedirectURL:  opts.BaseURL + "/auth/google/callback",
			Scopes:       []string{oidc.ScopeOpenID, "email", "profile"},
		},
		verifier: provider.Verifier(&oidc.Config{ClientID: opts.GoogleClientID}),
	}, nil
}

func (s *Service) handleGoogleStart(w http.ResponseWriter, r *http.Request) {
	state := randomToken()
	pkceVerifier := oauth2.GenerateVerifier()

	// Short-lived cookies carry the CSRF state and PKCE verifier across the
	// redirect round-trip.
	for name, value := range map[string]string{stateCookie: state, verifierCookie: pkceVerifier} {
		http.SetCookie(w, &http.Cookie{
			Name:     name,
			Value:    value,
			Path:     "/auth/google",
			MaxAge:   600,
			HttpOnly: true,
			Secure:   s.secureCookies,
			SameSite: http.SameSiteLaxMode,
		})
	}

	http.Redirect(w, r, s.google.oauth.AuthCodeURL(state, oauth2.S256ChallengeOption(pkceVerifier)), http.StatusFound)
}

func (s *Service) handleGoogleCallback(w http.ResponseWriter, r *http.Request) {
	clearOauthCookies(w, s.secureCookies)

	stateC, err1 := r.Cookie(stateCookie)
	verifierC, err2 := r.Cookie(verifierCookie)
	if err1 != nil || err2 != nil || r.URL.Query().Get("state") != stateC.Value {
		http.Error(w, "invalid oauth state", http.StatusBadRequest)
		return
	}
	if errMsg := r.URL.Query().Get("error"); errMsg != "" {
		http.Error(w, "google sign-in declined: "+errMsg, http.StatusBadRequest)
		return
	}

	token, err := s.google.oauth.Exchange(r.Context(), r.URL.Query().Get("code"), oauth2.VerifierOption(verifierC.Value))
	if err != nil {
		slog.Error("oauth exchange", "err", err)
		http.Error(w, "google sign-in failed", http.StatusBadGateway)
		return
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		http.Error(w, "google sign-in failed: no id_token", http.StatusBadGateway)
		return
	}
	idToken, err := s.google.verifier.Verify(r.Context(), rawIDToken)
	if err != nil {
		slog.Error("id token verify", "err", err)
		http.Error(w, "google sign-in failed", http.StatusBadGateway)
		return
	}

	var claims struct {
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
		Picture       string `json:"picture"`
	}
	if err := idToken.Claims(&claims); err != nil || claims.Email == "" {
		http.Error(w, "google sign-in failed: bad claims", http.StatusBadGateway)
		return
	}
	if !claims.EmailVerified {
		http.Error(w, "google account email is not verified", http.StatusForbidden)
		return
	}

	user, err := s.upsertGoogleUser(r.Context(), idToken.Subject, claims.Email, claims.Name, claims.Picture)
	if errors.Is(err, ErrEmailNotAllowed) {
		http.Error(w, "this Waypoint instance does not allow sign-ups for "+claims.Email, http.StatusForbidden)
		return
	}
	if err != nil {
		slog.Error("upsert google user", "err", err)
		http.Error(w, "sign-in failed", http.StatusInternalServerError)
		return
	}
	if err := s.signIn(w, r, user); err != nil {
		slog.Error("create session", "err", err)
		http.Error(w, "sign-in failed", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusFound)
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

func clearOauthCookies(w http.ResponseWriter, secure bool) {
	for _, name := range []string{stateCookie, verifierCookie} {
		http.SetCookie(w, &http.Cookie{
			Name: name, Value: "", Path: "/auth/google", MaxAge: -1,
			HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode,
		})
	}
}
