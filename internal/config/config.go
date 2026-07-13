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
	// LocalAuth enables email/password sign-in (intended for dev/testing).
	LocalAuth bool
	// AllowedEmails restricts who may sign up beyond the first user. Empty
	// means the instance is closed after the first sign-in.
	AllowedEmails []string
	// NominatimURL is the geocoding endpoint for place search. Defaults to
	// the public OSM instance; self-hosters with heavy usage should point
	// this at their own.
	NominatimURL string
	// Language is the preferred language for geocoded place names
	// (BCP 47, e.g. "en" or "fr"). Empty keeps native names.
	Language string
	// TileURL is the raster tile URL template ({z}/{x}/{y}) for map views.
	// Defaults to OpenStreetMap; point it at your own tile server or a
	// commercial provider for heavier usage.
	TileURL string
	// MapStyleURL is an optional MapLibre vector style JSON URL (MapTiler,
	// self-hosted Protomaps, …). When set it takes precedence over TileURL
	// and map labels localize to Language.
	MapStyleURL string
	// DataDir is where uploaded files (journal photos) are stored. The
	// docker image mounts a volume at /data.
	DataDir string
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
		LocalAuth:          os.Getenv("WAYPOINT_LOCAL_AUTH") == "true",
		AllowedEmails:      splitList(os.Getenv("WAYPOINT_ALLOWED_EMAILS")),
		NominatimURL:       strings.TrimSuffix(getenv("WAYPOINT_NOMINATIM_URL", "https://nominatim.openstreetmap.org"), "/"),
		Language:           os.Getenv("WAYPOINT_LANGUAGE"),
		TileURL:            getenv("WAYPOINT_TILE_URL", "https://tile.openstreetmap.org/{z}/{x}/{y}.png"),
		MapStyleURL:        os.Getenv("WAYPOINT_MAP_STYLE_URL"),
		DataDir:            getenv("WAYPOINT_DATA_DIR", "data"),
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("WAYPOINT_DATABASE_URL is required")
	}
	if (cfg.GoogleClientID == "") != (cfg.GoogleClientSecret == "") {
		return Config{}, fmt.Errorf("WAYPOINT_GOOGLE_CLIENT_ID and WAYPOINT_GOOGLE_CLIENT_SECRET must be set together")
	}
	return cfg, nil
}

func splitList(s string) []string {
	var out []string
	for _, v := range strings.Split(s, ",") {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
