// Package auth implements sign-in (Google OIDC, and local accounts in #3),
// server-side sessions, and the middleware that resolves the session cookie
// to a user. See docs/ARCHITECTURE.md → Authentication.
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
	google   *googleProvider // nil when Google sign-in is not configured
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
		s.google = g
	}
	return s, nil
}

type Options struct {
	// BaseURL is the public URL of the instance; redirect URIs derive from it.
	BaseURL            string
	GoogleClientID     string
	GoogleClientSecret string
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
		mux.HandleFunc("GET /auth/google", s.handleGoogleStart)
		mux.HandleFunc("GET /auth/google/callback", s.handleGoogleCallback)
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
	if s.localAuth {
		providers = append(providers, "local")
	}
	writeJSON(w, http.StatusOK, map[string]any{"providers": providers})
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
