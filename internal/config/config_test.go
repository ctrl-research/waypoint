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
