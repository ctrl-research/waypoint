// Package store contains Postgres-backed data access, one type per
// aggregate. Queries are written in internal/store/queries/*.sql and
// compiled by sqlc into internal/store/sqlcgen (`make generate`); this
// package is a thin façade that owns transactions and error translation.
package store

import (
	"errors"

	"github.com/jackc/pgx/v5"
)

// ErrNotFound is returned when a query matches no rows.
var ErrNotFound = errors.New("not found")

// translate maps driver-level sentinel errors to store-level ones.
func translate(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
