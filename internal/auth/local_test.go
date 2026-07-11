package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ctrl-research/waypoint/internal/store"
	"github.com/ctrl-research/waypoint/internal/store/storetest"
)

func TestLocalLogin(t *testing.T) {
	pool := storetest.Pool(t)
	users := store.NewUsers(pool)
	svc, err := NewService(context.Background(), users, store.NewSessions(pool), Options{
		BaseURL:   "http://localhost:8080",
		LocalAuth: true,
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	hash, err := HashPassword("correct-horse")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if _, err := users.Create(context.Background(), store.CreateUserParams{
		Email: "local@example.com", PasswordHash: &hash,
	}); err != nil {
		t.Fatalf("create user: %v", err)
	}
	if _, err := users.Create(context.Background(), store.CreateUserParams{
		Email: "googleonly@example.com", GoogleSub: strPtr("sub-google-only"),
	}); err != nil {
		t.Fatalf("create google user: %v", err)
	}

	login := func(body string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		svc.handleLogin(rec, httptest.NewRequest("POST", "/auth/login", strings.NewReader(body)))
		return rec
	}

	t.Run("valid credentials set a session cookie", func(t *testing.T) {
		rec := login(`{"email":"local@example.com","password":"correct-horse"}`)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
		}
		cookies := rec.Result().Cookies()
		if len(cookies) != 1 || cookies[0].Name != "waypoint_session" {
			t.Fatalf("expected session cookie, got %+v", cookies)
		}
	})

	for name, body := range map[string]string{
		"wrong password":      `{"email":"local@example.com","password":"wrong"}`,
		"unknown email":       `{"email":"ghost@example.com","password":"whatever"}`,
		"google-only account": `{"email":"googleonly@example.com","password":"whatever"}`,
	} {
		t.Run(name+" rejected with 401", func(t *testing.T) {
			if rec := login(body); rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401", rec.Code)
			}
		})
	}

	t.Run("missing fields rejected with 400", func(t *testing.T) {
		if rec := login(`{"email":"local@example.com"}`); rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rec.Code)
		}
	})

	t.Run("providers includes local", func(t *testing.T) {
		rec := httptest.NewRecorder()
		svc.handleProviders(rec, httptest.NewRequest("GET", "/auth/providers", nil))
		if !strings.Contains(rec.Body.String(), `"local"`) {
			t.Fatalf("providers = %s", rec.Body.String())
		}
	})
}

func TestAllowlist(t *testing.T) {
	pool := storetest.Pool(t)
	users := store.NewUsers(pool)
	svc, err := NewService(context.Background(), users, store.NewSessions(pool), Options{
		BaseURL:       "http://localhost:8080",
		AllowedEmails: []string{"Invited@Example.com"},
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	ctx := context.Background()

	t.Run("first user allowed regardless of allowlist", func(t *testing.T) {
		first, err := svc.upsertGoogleUser(ctx, "sub-first", "founder@example.com", "Founder", "")
		if err != nil {
			t.Fatalf("upsert: %v", err)
		}
		if !first.IsAdmin {
			t.Fatal("first user should be admin")
		}
	})

	t.Run("uninvited sign-up rejected", func(t *testing.T) {
		if _, err := svc.upsertGoogleUser(ctx, "sub-stranger", "stranger@example.com", "Stranger", ""); !errors.Is(err, ErrEmailNotAllowed) {
			t.Fatalf("err = %v, want ErrEmailNotAllowed", err)
		}
	})

	t.Run("invited sign-up allowed, case-insensitive", func(t *testing.T) {
		if _, err := svc.upsertGoogleUser(ctx, "sub-invited", "invited@example.com", "Invited", ""); err != nil {
			t.Fatalf("upsert: %v", err)
		}
	})

	t.Run("existing user signs in even if not on allowlist", func(t *testing.T) {
		if _, err := svc.upsertGoogleUser(ctx, "sub-first", "founder@example.com", "Founder", ""); err != nil {
			t.Fatalf("existing user sign-in: %v", err)
		}
	})
}
