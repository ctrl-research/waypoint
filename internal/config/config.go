// Package config loads Waypoint configuration from WAYPOINT_* environment
// variables. Environment variables are the only configuration mechanism.
package config

import (
	"fmt"
	"os"
)

type Config struct {
	// Addr is the listen address, e.g. ":8080".
	Addr string
	// DatabaseURL is a postgres connection string.
	DatabaseURL string
}

func Load() (Config, error) {
	cfg := Config{
		Addr:        getenv("WAYPOINT_ADDR", ":8080"),
		DatabaseURL: os.Getenv("WAYPOINT_DATABASE_URL"),
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("WAYPOINT_DATABASE_URL is required")
	}
	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
