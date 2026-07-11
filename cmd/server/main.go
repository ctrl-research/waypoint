package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ctrl-research/waypoint/internal/auth"
	"github.com/ctrl-research/waypoint/internal/config"
	"github.com/ctrl-research/waypoint/internal/geocode"
	"github.com/ctrl-research/waypoint/internal/server"
	"github.com/ctrl-research/waypoint/internal/store"
	"github.com/ctrl-research/waypoint/migrations"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if len(os.Args) > 1 && os.Args[1] == "seed" {
		return runSeed(ctx, os.Args[2:])
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect postgres: %w", err)
	}
	defer pool.Close()

	if err := waitForDB(ctx, pool); err != nil {
		return fmt.Errorf("postgres not reachable: %w", err)
	}

	if err := migrations.Up(ctx, cfg.DatabaseURL); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	authSvc, err := auth.NewService(ctx, store.NewUsers(pool), store.NewSessions(pool), auth.Options{
		BaseURL:            cfg.BaseURL,
		GoogleClientID:     cfg.GoogleClientID,
		GoogleClientSecret: cfg.GoogleClientSecret,
		LocalAuth:          cfg.LocalAuth,
		AllowedEmails:      cfg.AllowedEmails,
	})
	if err != nil {
		return fmt.Errorf("auth: %w", err)
	}
	if !cfg.GoogleEnabled() {
		slog.Warn("google sign-in not configured (set WAYPOINT_GOOGLE_CLIENT_ID/SECRET)")
	}
	go authSvc.GCLoop(ctx)

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           server.New(pool, authSvc, geocode.New(cfg.NominatimURL), server.Options{TileURL: cfg.TileURL}),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errc := make(chan error, 1)
	go func() {
		slog.Info("listening", "addr", cfg.Addr)
		errc <- srv.ListenAndServe()
	}()

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		slog.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}

// waitForDB retries pings so the app can start alongside postgres in compose
// without ordering hacks.
func waitForDB(ctx context.Context, pool *pgxpool.Pool) error {
	var lastErr error
	for i := 0; i < 30; i++ {
		pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		lastErr = pool.Ping(pingCtx)
		cancel()
		if lastErr == nil {
			return nil
		}
		if errors.Is(ctx.Err(), context.Canceled) {
			return ctx.Err()
		}
		time.Sleep(time.Second)
	}
	return lastErr
}
