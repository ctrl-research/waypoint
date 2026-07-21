package config

import "testing"

func TestLoad(t *testing.T) {
	t.Run("requires database url", func(t *testing.T) {
		t.Setenv("WAYPOINT_DATABASE_URL", "")
		if _, err := Load(); err == nil {
			t.Fatal("expected error when WAYPOINT_DATABASE_URL is unset")
		}
	})

	t.Run("defaults addr", func(t *testing.T) {
		t.Setenv("WAYPOINT_DATABASE_URL", "postgres://localhost/waypoint")
		t.Setenv("WAYPOINT_ADDR", "")
		cfg, err := Load()
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Addr != ":8080" {
			t.Fatalf("Addr = %q, want %q", cfg.Addr, ":8080")
		}
	})
}

// The OIDC issuer must be preserved byte-for-byte: providers like Authentik
// publish issuers with a trailing slash, and go-oidc requires an exact
// match with the discovery document (#108).
func TestOIDCIssuerNotNormalized(t *testing.T) {
	t.Setenv("WAYPOINT_DATABASE_URL", "postgres://localhost/waypoint")
	t.Setenv("WAYPOINT_OIDC_ISSUER_URL", "https://sso.example.com/application/o/waypoint/")
	t.Setenv("WAYPOINT_OIDC_CLIENT_ID", "id")
	t.Setenv("WAYPOINT_OIDC_CLIENT_SECRET", "secret")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.OIDCIssuerURL != "https://sso.example.com/application/o/waypoint/" {
		t.Fatalf("issuer = %q, trailing slash must be preserved", cfg.OIDCIssuerURL)
	}
}
