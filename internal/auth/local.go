package auth

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"golang.org/x/crypto/bcrypt"

	"github.com/ctrl-research/waypoint/internal/store"
)

// handleLogin signs in an email/password account. Registered only when
// WAYPOINT_LOCAL_AUTH=true; local users are created with the seed command
// (there is deliberately no self-registration endpoint).
func (s *Service) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, errorBody("bad_request", "email and password are required"))
		return
	}

	user, err := s.users.ByEmail(r.Context(), req.Email)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		slog.Error("local login", "err", err)
		writeJSON(w, http.StatusInternalServerError, errorBody("internal", "sign-in failed"))
		return
	}
	// Same response for unknown email, google-only account, and wrong
	// password — don't leak which accounts exist.
	if err != nil || user.PasswordHash == nil ||
		bcrypt.CompareHashAndPassword([]byte(*user.PasswordHash), []byte(req.Password)) != nil {
		writeJSON(w, http.StatusUnauthorized, errorBody("invalid_credentials", "invalid email or password"))
		return
	}

	if err := s.signIn(w, r, user); err != nil {
		slog.Error("create session", "err", err)
		writeJSON(w, http.StatusInternalServerError, errorBody("internal", "sign-in failed"))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func errorBody(code, message string) map[string]any {
	return map[string]any{"error": map[string]string{"code": code, "message": message}}
}

// HashPassword is used by the seed command; bcrypt keeps local auth simple
// and its cost factor is fine for a dev/testing credential path.
func HashPassword(password string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(h), err
}
