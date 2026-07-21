package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/ctrl-research/waypoint/internal/store"
)

// ssoProvider is one OIDC identity provider — Google, or the generic
// WAYPOINT_OIDC_* one (Authentik, Keycloak, …). Both share the same
// authorization-code + PKCE flow; they differ only in discovery issuer,
// how strict email verification is, and which subject column links users.
type ssoProvider struct {
	// key names the routes (/auth/{key}, /auth/{key}/callback) and cookies.
	key string
	// display is shown on the login button ("Google", "Authentik", …).
	display  string
	oauth    oauth2.Config
	verifier *oidc.IDTokenVerifier
	// requireVerifiedEmail rejects tokens without email_verified=true.
	// Google sets the claim reliably; self-hosted IdPs often omit it, so
	// the generic provider only rejects an explicit false.
	requireVerifiedEmail bool
	// upsert resolves the token's identity to a Waypoint user.
	upsert func(ctx context.Context, sub, email, name, picture string) (store.User, error)
}

func newSSOProvider(ctx context.Context, key, display, issuer, clientID, clientSecret, baseURL string, requireVerified bool) (*ssoProvider, error) {
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		// Issuers are exact-match identifiers and providers disagree on
		// trailing slashes (Authentik ends with one, Keycloak doesn't).
		// When the ONLY difference is that slash, discovery has already
		// proven which form is canonical — retry with it rather than
		// crash-looping over one character (#108).
		alt := strings.TrimSuffix(issuer, "/")
		if alt == issuer {
			alt = issuer + "/"
		}
		if p2, err2 := oidc.NewProvider(ctx, alt); err2 == nil {
			slog.Warn("oidc issuer normalized to the provider's canonical form — update the configured issuer",
				"provider", key, "configured", issuer, "canonical", alt)
			provider, err = p2, nil
			issuer = alt
		}
	}
	if err != nil {
		return nil, fmt.Errorf("%s oidc discovery: %w", key, err)
	}
	return &ssoProvider{
		key:     key,
		display: display,
		oauth: oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Endpoint:     provider.Endpoint(),
			RedirectURL:  baseURL + "/auth/" + key + "/callback",
			Scopes:       []string{oidc.ScopeOpenID, "email", "profile"},
		},
		verifier:             provider.Verifier(&oidc.Config{ClientID: clientID}),
		requireVerifiedEmail: requireVerified,
	}, nil
}

func (p *ssoProvider) stateCookie() string    { return "waypoint_oauth_state_" + p.key }
func (p *ssoProvider) verifierCookie() string { return "waypoint_oauth_verifier_" + p.key }

func (s *Service) handleSSOStart(p *ssoProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		state := randomToken()
		pkceVerifier := oauth2.GenerateVerifier()

		// Short-lived cookies carry the CSRF state and PKCE verifier across
		// the redirect round-trip.
		for name, value := range map[string]string{p.stateCookie(): state, p.verifierCookie(): pkceVerifier} {
			http.SetCookie(w, &http.Cookie{
				Name:     name,
				Value:    value,
				Path:     "/auth/" + p.key,
				MaxAge:   600,
				HttpOnly: true,
				Secure:   s.secureCookies,
				SameSite: http.SameSiteLaxMode,
			})
		}

		http.Redirect(w, r, p.oauth.AuthCodeURL(state, oauth2.S256ChallengeOption(pkceVerifier)), http.StatusFound)
	}
}

func (s *Service) handleSSOCallback(p *ssoProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		clearSSOCookies(w, p, s.secureCookies)

		stateC, err1 := r.Cookie(p.stateCookie())
		verifierC, err2 := r.Cookie(p.verifierCookie())
		if err1 != nil || err2 != nil || r.URL.Query().Get("state") != stateC.Value {
			http.Error(w, "invalid oauth state", http.StatusBadRequest)
			return
		}
		if errMsg := r.URL.Query().Get("error"); errMsg != "" {
			http.Error(w, p.display+" sign-in declined: "+errMsg, http.StatusBadRequest)
			return
		}

		token, err := p.oauth.Exchange(r.Context(), r.URL.Query().Get("code"), oauth2.VerifierOption(verifierC.Value))
		if err != nil {
			slog.Error("oauth exchange", "provider", p.key, "err", err)
			http.Error(w, p.display+" sign-in failed", http.StatusBadGateway)
			return
		}
		rawIDToken, ok := token.Extra("id_token").(string)
		if !ok {
			http.Error(w, p.display+" sign-in failed: no id_token", http.StatusBadGateway)
			return
		}
		idToken, err := p.verifier.Verify(r.Context(), rawIDToken)
		if err != nil {
			slog.Error("id token verify", "provider", p.key, "err", err)
			http.Error(w, p.display+" sign-in failed", http.StatusBadGateway)
			return
		}

		var claims struct {
			Email             string `json:"email"`
			EmailVerified     *bool  `json:"email_verified"`
			Name              string `json:"name"`
			PreferredUsername string `json:"preferred_username"`
			Picture           string `json:"picture"`
		}
		if err := idToken.Claims(&claims); err != nil || claims.Email == "" {
			http.Error(w, p.display+" sign-in failed: bad claims", http.StatusBadGateway)
			return
		}
		verified := claims.EmailVerified != nil && *claims.EmailVerified
		if p.requireVerifiedEmail && !verified {
			http.Error(w, p.display+" account email is not verified", http.StatusForbidden)
			return
		}
		if claims.EmailVerified != nil && !*claims.EmailVerified && !p.requireVerifiedEmail {
			// Even lenient providers must not assert an unverified email.
			http.Error(w, p.display+" account email is not verified", http.StatusForbidden)
			return
		}
		name := claims.Name
		if name == "" {
			name = claims.PreferredUsername
		}

		user, err := p.upsert(r.Context(), idToken.Subject, claims.Email, name, claims.Picture)
		if errors.Is(err, ErrEmailNotAllowed) {
			http.Error(w, "this Waypoint instance does not allow sign-ups for "+claims.Email, http.StatusForbidden)
			return
		}
		if err != nil {
			slog.Error("upsert sso user", "provider", p.key, "err", err)
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
}

func clearSSOCookies(w http.ResponseWriter, p *ssoProvider, secure bool) {
	for _, name := range []string{p.stateCookie(), p.verifierCookie()} {
		http.SetCookie(w, &http.Cookie{
			Name: name, Value: "", Path: "/auth/" + p.key, MaxAge: -1,
			HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode,
		})
	}
}

// upsertOIDCUser mirrors upsertGoogleUser for the generic provider: find by
// subject, link an existing account with the same email, or create a user
// subject to the allowlist.
func (s *Service) upsertOIDCUser(ctx context.Context, sub, email, name, picture string) (store.User, error) {
	var avatar *string
	if picture != "" {
		avatar = &picture
	}

	user, err := s.users.ByOIDCSub(ctx, sub)
	switch {
	case err == nil:
		return s.users.UpdateProfile(ctx, user.ID, name, avatar)
	case !errors.Is(err, store.ErrNotFound):
		return store.User{}, err
	}

	if user, err = s.users.ByEmail(ctx, email); err == nil {
		return s.users.LinkOIDC(ctx, user.ID, sub, name, avatar)
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
		OidcSub:     &sub,
		IsAdmin:     count == 0,
	})
}
