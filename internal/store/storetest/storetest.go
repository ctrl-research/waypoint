// Package storetest provides a real-Postgres pool for store-layer tests,
// per the convention in docs/ARCHITECTURE.md (no database mocks).
package storetest

import (
	"context"
	"net/url"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ctrl-research/waypoint/migrations"
)

// Pool connects to WAYPOINT_TEST_DATABASE_URL, creating the database and
// applying migrations if needed, and truncates all tables so each test starts
// clean. Tests are skipped when the variable is unset (e.g. `go test` without
// a local postgres). CI and `make test-db` set it.
func Pool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dbURL := os.Getenv("WAYPOINT_TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("WAYPOINT_TEST_DATABASE_URL not set; skipping postgres-backed tests")
	}

	ctx := context.Background()
	ensureDatabase(t, ctx, dbURL)

	if err := migrations.Up(ctx, dbURL); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}
	t.Cleanup(pool.Close)

	if _, err := pool.Exec(ctx, `TRUNCATE users CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	return pool
}

// ensureDatabase creates the target database if it does not exist, by
// connecting to the maintenance database on the same server.
func ensureDatabase(t *testing.T, ctx context.Context, dbURL string) {
	t.Helper()

	u, err := url.Parse(dbURL)
	if err != nil {
		t.Fatalf("parse WAYPOINT_TEST_DATABASE_URL: %v", err)
	}
	dbName := strings.TrimPrefix(u.Path, "/")
	if dbName == "" {
		t.Fatal("WAYPOINT_TEST_DATABASE_URL has no database name")
	}
	if !strings.HasSuffix(dbName, "_test") {
		t.Fatalf("refusing to run against %q: test database name must end in _test", dbName)
	}

	maint := *u
	maint.Path = path.Join("/", "postgres")
	conn, err := pgx.Connect(ctx, maint.String())
	if err != nil {
		t.Fatalf("connect maintenance db: %v", err)
	}
	defer conn.Close(ctx)

	var exists bool
	if err := conn.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)`, dbName).Scan(&exists); err != nil {
		t.Fatalf("check database exists: %v", err)
	}
	if !exists {
		if _, err := conn.Exec(ctx, `CREATE DATABASE `+pgx.Identifier{dbName}.Sanitize()); err != nil {
			t.Fatalf("create test database: %v", err)
		}
	}
}
