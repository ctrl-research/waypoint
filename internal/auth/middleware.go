package auth

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/ctrl-research/waypoint/internal/store"
)

type ctxKey struct{}

// WithUser resolves the session cookie (when present and valid) and attaches
// the user to the request context. It never rejects a request — pair with
// RequireUser for protected routes.
func (s *Service) WithUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(sessionCookie)
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}
		user, err := s.sessions.UserByToken(r.Context(), hashToken(c.Value))
		if err != nil {
			if !errors.Is(err, store.ErrNotFound) {
				slog.Error("resolve session", "err", err)
			}
			next.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, user)))
	})
}

// RequireUser rejects requests that have no signed-in user with 401.
func RequireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := UserFrom(r.Context()); !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]any{
				"error": map[string]string{"code": "unauthenticated", "message": "sign in required"},
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// WithUserContext attaches a user resolved outside the session flow —
// bearer-token surfaces like the MCP endpoint (#92).
func WithUserContext(ctx context.Context, user store.User) context.Context {
	return context.WithValue(ctx, ctxKey{}, user)
}

// UserFrom returns the signed-in user attached by WithUser, if any.
func UserFrom(ctx context.Context) (store.User, bool) {
	user, ok := ctx.Value(ctxKey{}).(store.User)
	return user, ok
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
