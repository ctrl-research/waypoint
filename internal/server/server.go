// Package server wires the HTTP router, middleware, and handlers.
package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ctrl-research/waypoint/internal/auth"
	"github.com/ctrl-research/waypoint/internal/geocode"
	"github.com/ctrl-research/waypoint/internal/photos"
	"github.com/ctrl-research/waypoint/internal/store"
	"github.com/ctrl-research/waypoint/internal/webui"
)

// Options carries instance configuration the server needs at runtime.
type Options struct {
	// TileURL is the raster tile URL template exposed to map views.
	TileURL string
	// DataDir is where uploaded files are stored.
	DataDir string
}

func New(pool *pgxpool.Pool, authSvc *auth.Service, geo *geocode.Client, opts Options) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", handleHealthz(pool))
	mux.HandleFunc("GET /api/v1/ping", handlePing)
	mux.Handle("GET /api/v1/me", auth.RequireUser(http.HandlerFunc(handleMe)))
	mux.Handle("GET /api/v1/geocode", auth.RequireUser(handleGeocode(geo)))
	mux.Handle("GET /api/v1/config", auth.RequireUser(handleConfig(opts)))
	(&tripsAPI{trips: store.NewTrips(pool), photos: photos.NewStore(opts.DataDir)}).routes(mux)
	authSvc.Routes(mux)
	mux.Handle("/", webui.Handler())

	return logRequests(authSvc.WithUser(mux))
}

// handleConfig exposes instance settings the SPA needs (currently the map
// tile template).
func handleConfig(opts Options) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"tileUrl": opts.TileURL})
	}
}

func handleHealthz(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := pool.Ping(ctx); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "degraded", "database": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

func handlePing(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"message": "pong"})
}

func handleMe(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFrom(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{
		"id":          user.ID,
		"email":       user.Email,
		"displayName": user.DisplayName,
		"avatarUrl":   user.AvatarURL,
		"isAdmin":     user.IsAdmin,
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		slog.Info("request", "method", r.Method, "path", r.URL.Path, "duration", time.Since(start))
	})
}
