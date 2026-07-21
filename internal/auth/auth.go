// Package auth implements sign-in (Google OIDC, a generic OIDC provider,
// and local accounts), server-side sessions, and the middleware that
// resolves the session cookie to a user. See docs/ARCHITECTURE.md →
// Authentication.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/ctrl-research/waypoint/internal/store"
)

const (
	sessionCookie = "waypoint_session"
	// SessionTTL is how long a session lives without re-authenticating.
	SessionTTL = 30 * 24 * time.Hour
)

type Service struct {
	users    *store.Users
	sessions *store.Sessions
	google   *ssoProvider // nil when Google sign-in is not configured
	oidc     *ssoProvider // nil when no generic OIDC provider is configured
	// secureCookies is false only for plain-http development instances.
	secureCookies bool
	baseURL       string
	localAuth     bool
	// allowedEmails restricts sign-ups beyond the first user (lowercased).
	// Empty means the instance is closed once the first user exists.
	allowedEmails map[string]bool
}

func NewService(ctx context.Context, users *store.Users, sessions *store.Sessions, opts Options) (*Service, error) {
	allowed := make(map[string]bool, len(opts.AllowedEmails))
	for _, e := range opts.AllowedEmails {
		allowed[strings.ToLower(e)] = true
	}
	s := &Service{
		users:         users,
		sessions:      sessions,
		secureCookies: strings.HasPrefix(opts.BaseURL, "https://"),
		baseURL:       opts.BaseURL,
		localAuth:     opts.LocalAuth,
		allowedEmails: allowed,
	}
	if opts.GoogleClientID != "" {
		g, err := newGoogleProvider(ctx, opts)
		if err != nil {
			return nil, err
		}
		g.upsert = s.upsertGoogleUser
		s.google = g
	}
	if opts.OIDCIssuerURL != "" {
		name := opts.OIDCName
		if name == "" {
			name = "SSO"
		}
		// Self-hosted IdPs often omit email_verified; only an explicit
		// false is rejected (see handleSSOCallback).
		p, err := newSSOProvider(ctx, "oidc", name, opts.OIDCIssuerURL, opts.OIDCClientID, opts.OIDCClientSecret, opts.BaseURL, false)
		if err != nil {
			return nil, err
		}
		p.upsert = s.upsertOIDCUser
		s.oidc = p
	}
	return s, nil
}

type Options struct {
	// BaseURL is the public URL of the instance; redirect URIs derive from it.
	BaseURL            string
	GoogleClientID     string
	GoogleClientSecret string
	// Generic OIDC provider (Authentik, Keycloak, …): issuer URL for
	// discovery, client credentials, and a display name for the login
	// button. Issuer, ID, and secret must be set together.
	OIDCIssuerURL    string
	OIDCClientID     string
	OIDCClientSecret string
	OIDCName         string
	// LocalAuth enables POST /auth/login for email/password accounts.
	LocalAuth bool
	// AllowedEmails restricts sign-ups beyond the first user.
	AllowedEmails []string
}

// Routes registers the /auth/* endpoints.
func (s *Service) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /auth/providers", s.handleProviders)
	mux.HandleFunc("POST /auth/logout", s.handleLogout)
	if s.google != nil {
		mux.HandleFunc("GET /auth/google", s.handleSSOStart(s.google))
		mux.HandleFunc("GET /auth/google/callback", s.handleSSOCallback(s.google))
	}
	if s.oidc != nil {
		mux.HandleFunc("GET /auth/oidc", s.handleSSOStart(s.oidc))
		mux.HandleFunc("GET /auth/oidc/callback", s.handleSSOCallback(s.oidc))
	}
	if s.localAuth {
		mux.HandleFunc("POST /auth/login", s.handleLogin)
	}
}

// handleProviders tells the frontend which sign-in methods to offer.
func (s *Service) handleProviders(w http.ResponseWriter, r *http.Request) {
	providers := []string{}
	if s.google != nil {
		providers = append(providers, "google")
	}
	oidcName := ""
	if s.oidc != nil {
		providers = append(providers, "oidc")
		oidcName = s.oidc.display
	}
	if s.localAuth {
		providers = append(providers, "local")
	}
	writeJSON(w, http.StatusOK, map[string]any{"providers": providers, "oidcName": oidcName})
}

// emailAllowed reports whether email may create a new account. The first
// user on an instance is always allowed (and becomes admin); see
// upsertGoogleUser.
func (s *Service) emailAllowed(email string) bool {
	return s.allowedEmails[strings.ToLower(email)]
}

// signIn creates a session for the user and sets the cookie.
func (s *Service) signIn(w http.ResponseWriter, r *http.Request, user store.User) error {
	token := randomToken()
	if err := s.sessions.Create(r.Context(), hashToken(token), user.ID, time.Now().Add(SessionTTL)); err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		MaxAge:   int(SessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   s.secureCookies,
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

func (s *Service) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		if err := s.sessions.Delete(r.Context(), hashToken(c.Value)); err != nil {
			slog.Error("delete session", "err", err)
		}
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.secureCookies,
		SameSite: http.SameSiteLaxMode,
	})
	w.WriteHeader(http.StatusNoContent)
}

// GCLoop periodically deletes expired sessions until ctx is cancelled.
func (s *Service) GCLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if n, err := s.sessions.DeleteExpired(ctx); err != nil {
				slog.Error("session gc", "err", err)
			} else if n > 0 {
				slog.Info("session gc", "deleted", n)
			}
		}
	}
}

func randomToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func hashToken(token string) []byte {
	h := sha256.Sum256([]byte(token))
	return h[:]
}
