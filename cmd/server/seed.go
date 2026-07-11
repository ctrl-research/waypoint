package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ctrl-research/waypoint/internal/auth"
	"github.com/ctrl-research/waypoint/internal/config"
	"github.com/ctrl-research/waypoint/internal/store"
	"github.com/ctrl-research/waypoint/migrations"
)

// runSeed implements `waypoint seed`: idempotently create (or reset the
// password of) a local email/password user for development. Pair with
// WAYPOINT_LOCAL_AUTH=true to sign in with it.
func runSeed(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("seed", flag.ExitOnError)
	email := fs.String("email", "dev@waypoint.local", "email of the local user")
	password := fs.String("password", "waypoint-dev", "password of the local user")
	admin := fs.Bool("admin", true, "make the user an admin")
	if err := fs.Parse(args); err != nil {
		return err
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

	if err := migrations.Up(ctx, cfg.DatabaseURL); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	hash, err := auth.HashPassword(*password)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	users := store.NewUsers(pool)
	user, err := users.ByEmail(ctx, *email)
	switch {
	case err == nil:
		if _, err := users.SetPassword(ctx, user.ID, hash); err != nil {
			return fmt.Errorf("update password: %w", err)
		}
		slog.Info("seed: password reset for existing user", "email", *email)
	case errors.Is(err, store.ErrNotFound):
		if _, err := users.Create(ctx, store.CreateUserParams{
			Email:        *email,
			DisplayName:  "Dev User",
			PasswordHash: &hash,
			IsAdmin:      *admin,
		}); err != nil {
			return fmt.Errorf("create user: %w", err)
		}
		slog.Info("seed: created local user", "email", *email, "admin", *admin)
	default:
		return fmt.Errorf("lookup user: %w", err)
	}
	return nil
}
