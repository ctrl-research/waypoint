// Package config loads Waypoint configuration from WAYPOINT_* environment
// variables. Environment variables are the only configuration mechanism.
package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	// Addr is the listen address, e.g. ":8080".
	Addr string
	// DatabaseURL is a postgres connection string.
	DatabaseURL string
	// BaseURL is the public URL of this instance, e.g.
	// "https://waypoint.example.com". Required when Google auth is enabled;
	// OAuth redirect URIs are derived from it.
	BaseURL string
	// GoogleClientID / GoogleClientSecret enable "Sign in with Google" when
	// both are set.
	GoogleClientID     string
	GoogleClientSecret string
}

// GoogleEnabled reports whether Google sign-in is configured.
func (c Config) GoogleEnabled() bool {
	return c.GoogleClientID != "" && c.GoogleClientSecret != ""
}

func Load() (Config, error) {
	cfg := Config{
		Addr:               getenv("WAYPOINT_ADDR", ":8080"),
		DatabaseURL:        os.Getenv("WAYPOINT_DATABASE_URL"),
		BaseURL:            strings.TrimSuffix(getenv("WAYPOINT_BASE_URL", "http://localhost:8080"), "/"),
		GoogleClientID:     os.Getenv("WAYPOINT_GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("WAYPOINT_GOOGLE_CLIENT_SECRET"),
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("WAYPOINT_DATABASE_URL is required")
	}
	if (cfg.GoogleClientID == "") != (cfg.GoogleClientSecret == "") {
		return Config{}, fmt.Errorf("WAYPOINT_GOOGLE_CLIENT_ID and WAYPOINT_GOOGLE_CLIENT_SECRET must be set together")
	}
	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
