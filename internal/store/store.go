// Package store contains Postgres-backed data access, one type per
// aggregate. Queries are hand-written SQL on a pgx pool; see
// docs/ARCHITECTURE.md (sqlc adoption is planned for M2, #8).
package store

import "errors"

// ErrNotFound is returned when a query matches no rows.
var ErrNotFound = errors.New("not found")
