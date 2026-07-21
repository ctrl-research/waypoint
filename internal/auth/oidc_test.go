package auth

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/ctrl-research/waypoint/internal/store"
	"github.com/ctrl-research/waypoint/internal/store/storetest"
)

// fakeIdP is a minimal OIDC provider: discovery, JWKS, and a token endpoint
// that mints RS256 id_tokens for whatever claims the test sets next.
type fakeIdP struct {
	srv    *httptest.Server
	key    *rsa.PrivateKey
	claims map[string]any
	// issuer overrides the advertised issuer (default srv.URL) — lets tests
	// model Authentik-style issuers that end with a trailing slash.
	issuer string
}

func newFakeIdP(t *testing.T) *fakeIdP {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa key: %v", err)
	}
	idp := &fakeIdP{key: key}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		issuer := idp.srv.URL
		if idp.issuer != "" {
			issuer = idp.issuer
		}
		json.NewEncoder(w).Encode(map[string]any{
			"issuer":                                issuer,
			"authorization_endpoint":                idp.srv.URL + "/authorize",
			"token_endpoint":                        idp.srv.URL + "/token",
			"jwks_uri":                              idp.srv.URL + "/jwks",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"keys": []map[string]any{{
				"kty": "RSA", "alg": "RS256", "use": "sig", "kid": "test",
				"n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
				"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
			}},
		})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token": "at",
			"token_type":   "Bearer",
			"id_token":     idp.signToken(t),
		})
	})
	idp.srv = httptest.NewServer(mux)
	t.Cleanup(idp.srv.Close)
	return idp
}

func (idp *fakeIdP) signToken(t *testing.T) string {
	t.Helper()
	header, _ := json.Marshal(map[string]string{"alg": "RS256", "kid": "test", "typ": "JWT"})
	payload, _ := json.Marshal(idp.claims)
	signing := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	digest := sha256.Sum256([]byte(signing))
	sig, err := rsa.SignPKCS1v15(rand.Reader, idp.key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return signing + "." + base64.RawURLEncoding.EncodeToString(sig)
}

// ssoRoundTrip runs the full start → callback flow and returns the response.
func ssoRoundTrip(t *testing.T, svc *Service) *httptest.ResponseRecorder {
	t.Helper()
	start := httptest.NewRecorder()
	svc.handleSSOStart(svc.oidc)(start, httptest.NewRequest("GET", "/auth/oidc", nil))
	if start.Code != http.StatusFound {
		t.Fatalf("start status = %d", start.Code)
	}
	loc, err := url.Parse(start.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parse authorize url: %v", err)
	}

	cb := httptest.NewRequest("GET", "/auth/oidc/callback?state="+loc.Query().Get("state")+"&code=any", nil)
	for _, c := range start.Result().Cookies() {
		cb.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	svc.handleSSOCallback(svc.oidc)(rec, cb)
	return rec
}

func TestGenericOIDC(t *testing.T) {
	idp := newFakeIdP(t)
	pool := storetest.Pool(t)
	users := store.NewUsers(pool)
	svc, err := NewService(context.Background(), users, store.NewSessions(pool), Options{
		BaseURL:          "http://localhost:8080",
		OIDCIssuerURL:    idp.srv.URL,
		OIDCClientID:     "waypoint-client",
		OIDCClientSecret: "shhh",
		OIDCName:         "Authentik",
		AllowedEmails:    []string{"friend@example.com"},
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	baseClaims := func(sub, email string) map[string]any {
		return map[string]any{
			"iss": idp.srv.URL, "aud": "waypoint-client", "sub": sub,
			"exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(),
			"email": email, "name": "Test Person",
		}
	}

	t.Run("providers advertises the display name", func(t *testing.T) {
		rec := httptest.NewRecorder()
		svc.handleProviders(rec, httptest.NewRequest("GET", "/auth/providers", nil))
		body := rec.Body.String()
		if !strings.Contains(body, `"oidc"`) || !strings.Contains(body, "Authentik") {
			t.Fatalf("providers = %s", body)
		}
	})

	t.Run("first sign-in creates the admin", func(t *testing.T) {
		idp.claims = baseClaims("oidc-sub-1", "owner@example.com")
		rec := ssoRoundTrip(t, svc)
		if rec.Code != http.StatusFound {
			t.Fatalf("callback = %d: %s", rec.Code, rec.Body.String())
		}
		user, err := users.ByOIDCSub(context.Background(), "oidc-sub-1")
		if err != nil {
			t.Fatalf("user not created: %v", err)
		}
		if !user.IsAdmin || user.Email != "owner@example.com" {
			t.Fatalf("user = %+v", user)
		}
		var gotSession bool
		for _, c := range rec.Result().Cookies() {
			if c.Name == sessionCookie && c.Value != "" {
				gotSession = true
			}
		}
		if !gotSession {
			t.Fatal("expected a session cookie")
		}
	})

	t.Run("allowlist gates new sign-ups", func(t *testing.T) {
		idp.claims = baseClaims("oidc-sub-2", "stranger@example.com")
		if rec := ssoRoundTrip(t, svc); rec.Code != http.StatusForbidden {
			t.Fatalf("stranger = %d, want 403", rec.Code)
		}
		idp.claims = baseClaims("oidc-sub-3", "friend@example.com")
		if rec := ssoRoundTrip(t, svc); rec.Code != http.StatusFound {
			t.Fatalf("friend = %d, want 302", rec.Code)
		}
	})

	t.Run("links an existing account by email", func(t *testing.T) {
		existing, err := users.Create(context.Background(), store.CreateUserParams{
			Email: "linked@example.com", DisplayName: "Linked", PasswordHash: strPtr("x"),
		})
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		idp.claims = baseClaims("oidc-sub-4", "linked@example.com")
		if rec := ssoRoundTrip(t, svc); rec.Code != http.StatusFound {
			t.Fatalf("link = %d, want 302", rec.Code)
		}
		got, err := users.ByOIDCSub(context.Background(), "oidc-sub-4")
		if err != nil || got.ID != existing.ID {
			t.Fatalf("expected link to existing user: %v %v", got.ID, err)
		}
	})

	t.Run("explicit unverified email is rejected", func(t *testing.T) {
		claims := baseClaims("oidc-sub-5", "friend@example.com")
		claims["email_verified"] = false
		idp.claims = claims
		if rec := ssoRoundTrip(t, svc); rec.Code != http.StatusForbidden {
			t.Fatalf("unverified = %d, want 403", rec.Code)
		}
	})
}

// Providers disagree on trailing slashes in issuer URLs (Authentik ends with
// one, Keycloak and Google don't), and the mismatch crash-loops the server
// over a single character (#108). Discovery already returns the canonical
// form, so newSSOProvider retries with the slash toggled.
func TestOIDCIssuerSlashSelfHeals(t *testing.T) {
	t.Run("extra slash in config is dropped", func(t *testing.T) {
		idp := newFakeIdP(t) // canonical issuer has no trailing slash
		if _, err := newSSOProvider(context.Background(), "oidc", "Test", idp.srv.URL+"/", "cid", "secret", "http://localhost:8080", false); err != nil {
			t.Fatalf("expected slash mismatch to self-heal: %v", err)
		}
	})

	t.Run("missing slash still signs in end to end", func(t *testing.T) {
		idp := newFakeIdP(t)
		idp.issuer = idp.srv.URL + "/" // Authentik-style canonical issuer

		pool := storetest.Pool(t)
		users := store.NewUsers(pool)
		svc, err := NewService(context.Background(), users, store.NewSessions(pool), Options{
			BaseURL:          "http://localhost:8080",
			OIDCIssuerURL:    idp.srv.URL, // configured WITHOUT the slash
			OIDCClientID:     "waypoint-client",
			OIDCClientSecret: "shhh",
			OIDCName:         "Authentik",
		})
		if err != nil {
			t.Fatalf("NewService: %v", err)
		}

		idp.claims = map[string]any{
			"iss": idp.issuer, "aud": "waypoint-client", "sub": "slash-sub-1",
			"exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(),
			"email": "healed@example.com", "name": "Healed",
		}
		if rec := ssoRoundTrip(t, svc); rec.Code != http.StatusFound {
			t.Fatalf("callback = %d: %s", rec.Code, rec.Body.String())
		}
		if _, err := users.ByOIDCSub(context.Background(), "slash-sub-1"); err != nil {
			t.Fatalf("user not created: %v", err)
		}
	})
}
