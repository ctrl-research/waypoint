package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/ctrl-research/waypoint/internal/store"
	"github.com/ctrl-research/waypoint/internal/store/storetest"
)

func newTestService(t *testing.T) (*Service, *store.Users) {
	t.Helper()
	pool := storetest.Pool(t)
	users := store.NewUsers(pool)
	svc, err := NewService(context.Background(), users, store.NewSessions(pool), Options{
		BaseURL: "http://localhost:8080",
		// Sign-ups beyond the first user require an allowlist entry
		// (closed-instance policy); TestAllowlist covers the rejection path.
		AllowedEmails: []string{"second@example.com", "local@example.com"},
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	return svc, users
}

func strPtr(s string) *string { return &s }

func TestUpsertGoogleUser(t *testing.T) {
	svc, users := newTestService(t)
	ctx := context.Background()

	t.Run("first user becomes admin, second does not", func(t *testing.T) {
		first, err := svc.upsertGoogleUser(ctx, "sub-1", "first@example.com", "First", "")
		if err != nil {
			t.Fatalf("upsert: %v", err)
		}
		if !first.IsAdmin {
			t.Fatal("first user should be admin")
		}
		second, err := svc.upsertGoogleUser(ctx, "sub-2", "second@example.com", "Second", "")
		if err != nil {
			t.Fatalf("upsert: %v", err)
		}
		if second.IsAdmin {
			t.Fatal("second user should not be admin")
		}
	})

	t.Run("repeat sign-in refreshes profile, keeps identity", func(t *testing.T) {
		again, err := svc.upsertGoogleUser(ctx, "sub-1", "first@example.com", "Renamed", "https://example.com/new.png")
		if err != nil {
			t.Fatalf("upsert: %v", err)
		}
		if again.DisplayName != "Renamed" || again.AvatarURL == nil {
			t.Fatalf("profile not refreshed: %+v", again)
		}
		n, _ := users.Count(ctx)
		if n != 2 {
			t.Fatalf("user count = %d, want 2 (no duplicate created)", n)
		}
	})

	t.Run("links existing account by email", func(t *testing.T) {
		local, err := users.Create(ctx, store.CreateUserParams{
			Email: "local@example.com", PasswordHash: strPtr("x"),
		})
		if err != nil {
			t.Fatalf("create local user: %v", err)
		}
		linked, err := svc.upsertGoogleUser(ctx, "sub-local", "local@example.com", "Local", "")
		if err != nil {
			t.Fatalf("upsert: %v", err)
		}
		if linked.ID != local.ID {
			t.Fatal("should link the existing account, not create a new one")
		}
		if linked.GoogleSub == nil || *linked.GoogleSub != "sub-local" {
			t.Fatalf("google_sub not linked: %+v", linked)
		}
	})
}

func TestSessionLifecycle(t *testing.T) {
	svc, users := newTestService(t)
	ctx := context.Background()

	user, err := users.Create(ctx, store.CreateUserParams{
		Email: "mw@example.com", GoogleSub: strPtr("sub-mw"),
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	// Protected endpoint: WithUser resolves the cookie, RequireUser guards.
	handler := svc.WithUser(RequireUser(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, _ := UserFrom(r.Context())
		fmt.Fprint(w, u.Email)
	})))

	t.Run("no cookie rejected", func(t *testing.T) {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest("GET", "/protected", nil))
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rec.Code)
		}
	})

	// Sign in and capture the session cookie.
	rec := httptest.NewRecorder()
	if err := svc.signIn(rec, httptest.NewRequest("GET", "/", nil), user); err != nil {
		t.Fatalf("signIn: %v", err)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != sessionCookie || !cookies[0].HttpOnly {
		t.Fatalf("expected one HttpOnly session cookie, got %+v", cookies)
	}
	session := cookies[0]

	t.Run("valid cookie resolves user", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/protected", nil)
		req.AddCookie(session)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK || rec.Body.String() != "mw@example.com" {
			t.Fatalf("status = %d body = %q", rec.Code, rec.Body.String())
		}
	})

	t.Run("garbage cookie rejected", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/protected", nil)
		req.AddCookie(&http.Cookie{Name: sessionCookie, Value: "forged"})
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rec.Code)
		}
	})

	t.Run("logout revokes the session", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/auth/logout", nil)
		req.AddCookie(session)
		rec := httptest.NewRecorder()
		svc.handleLogout(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("logout status = %d", rec.Code)
		}

		req = httptest.NewRequest("GET", "/protected", nil)
		req.AddCookie(session)
		rec = httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401 after logout", rec.Code)
		}
	})
}

// fakeIssuer serves just enough OIDC discovery for the start-of-flow test.
func fakeIssuer(t *testing.T) string {
	t.Helper()
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"issuer":                 srv.URL,
			"authorization_endpoint": srv.URL + "/authorize",
			"token_endpoint":         srv.URL + "/token",
			"jwks_uri":               srv.URL + "/jwks",
		})
	})
	return srv.URL
}

func TestGoogleStart(t *testing.T) {
	pool := storetest.Pool(t)

	issuerOverride = fakeIssuer(t)
	defer func() { issuerOverride = "" }()

	svc, err := NewService(context.Background(), store.NewUsers(pool), store.NewSessions(pool), Options{
		BaseURL:            "http://localhost:8080",
		GoogleClientID:     "client-id",
		GoogleClientSecret: "client-secret",
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	rec := httptest.NewRecorder()
	svc.handleGoogleStart(rec, httptest.NewRequest("GET", "/auth/google", nil))
	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", rec.Code)
	}

	loc, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parse redirect: %v", err)
	}
	q := loc.Query()
	if q.Get("client_id") != "client-id" {
		t.Fatalf("client_id = %q", q.Get("client_id"))
	}
	if q.Get("code_challenge_method") != "S256" || q.Get("code_challenge") == "" {
		t.Fatal("expected PKCE S256 challenge in authorize URL")
	}
	if q.Get("redirect_uri") != "http://localhost:8080/auth/google/callback" {
		t.Fatalf("redirect_uri = %q", q.Get("redirect_uri"))
	}

	var gotState bool
	for _, c := range rec.Result().Cookies() {
		if c.Name == stateCookie && c.Value == q.Get("state") && c.Value != "" {
			gotState = true
		}
	}
	if !gotState {
		t.Fatal("state cookie must match the state query parameter")
	}
}
