package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TripStatus string

const (
	TripPlanning  TripStatus = "planning"
	TripActive    TripStatus = "active"
	TripCompleted TripStatus = "completed"
)

func ValidTripStatus(s string) bool {
	switch TripStatus(s) {
	case TripPlanning, TripActive, TripCompleted:
		return true
	}
	return false
}

type Trip struct {
	ID          uuid.UUID
	OwnerID     uuid.UUID
	Title       string
	Description string
	Status      TripStatus
	StartDate   *time.Time
	EndDate     *time.Time
	CoverPhoto  *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type TripParams struct {
	Title       string
	Description string
	Status      TripStatus
	StartDate   *time.Time
	EndDate     *time.Time
	CoverPhoto  *string
}

type Trips struct {
	pool *pgxpool.Pool
}

func NewTrips(pool *pgxpool.Pool) *Trips {
	return &Trips{pool: pool}
}

const tripColumns = `id, owner_id, title, description, status, start_date, end_date, cover_photo, created_at, updated_at`

func scanTrip(row pgx.Row) (Trip, error) {
	var t Trip
	err := row.Scan(&t.ID, &t.OwnerID, &t.Title, &t.Description, &t.Status,
		&t.StartDate, &t.EndDate, &t.CoverPhoto, &t.CreatedAt, &t.UpdatedAt)
	if err == pgx.ErrNoRows {
		return Trip{}, ErrNotFound
	}
	return t, err
}

func (s *Trips) Create(ctx context.Context, ownerID uuid.UUID, p TripParams) (Trip, error) {
	return scanTrip(s.pool.QueryRow(ctx, `
		INSERT INTO trips (owner_id, title, description, status, start_date, end_date, cover_photo)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING `+tripColumns,
		ownerID, p.Title, p.Description, p.Status, p.StartDate, p.EndDate, p.CoverPhoto))
}

func (s *Trips) ByID(ctx context.Context, id uuid.UUID) (Trip, error) {
	return scanTrip(s.pool.QueryRow(ctx,
		`SELECT `+tripColumns+` FROM trips WHERE id = $1`, id))
}

func (s *Trips) ListByOwner(ctx context.Context, ownerID uuid.UUID) ([]Trip, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+tripColumns+` FROM trips
		WHERE owner_id = $1
		ORDER BY COALESCE(start_date, created_at::date) DESC, created_at DESC`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	trips := []Trip{}
	for rows.Next() {
		t, err := scanTrip(rows)
		if err != nil {
			return nil, err
		}
		trips = append(trips, t)
	}
	return trips, rows.Err()
}

// Update replaces all mutable fields; the handler merges PATCH semantics.
func (s *Trips) Update(ctx context.Context, id uuid.UUID, p TripParams) (Trip, error) {
	return scanTrip(s.pool.QueryRow(ctx, `
		UPDATE trips
		SET title = $2, description = $3, status = $4, start_date = $5,
		    end_date = $6, cover_photo = $7, updated_at = now()
		WHERE id = $1
		RETURNING `+tripColumns,
		id, p.Title, p.Description, p.Status, p.StartDate, p.EndDate, p.CoverPhoto))
}

func (s *Trips) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM trips WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// touch bumps the trip's updated_at when a child row changes.
func (s *Trips) touch(ctx context.Context, tripID uuid.UUID) {
	_, _ = s.pool.Exec(ctx, `UPDATE trips SET updated_at = now() WHERE id = $1`, tripID)
}
