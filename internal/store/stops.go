package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Stop struct {
	ID            uuid.UUID
	TripID        uuid.UUID
	Name          string
	Lat           *float64
	Lon           *float64
	ArrivalDate   *time.Time
	DepartureDate *time.Time
	Position      int32
	Notes         string
}

type StopParams struct {
	Name          string
	Lat           *float64
	Lon           *float64
	ArrivalDate   *time.Time
	DepartureDate *time.Time
	Notes         string
}

const stopColumns = `id, trip_id, name, lat, lon, arrival_date, departure_date, position, notes`

func scanStop(row pgx.Row) (Stop, error) {
	var st Stop
	err := row.Scan(&st.ID, &st.TripID, &st.Name, &st.Lat, &st.Lon,
		&st.ArrivalDate, &st.DepartureDate, &st.Position, &st.Notes)
	if err == pgx.ErrNoRows {
		return Stop{}, ErrNotFound
	}
	return st, err
}

// CreateStop appends the stop at the end of the trip's ordering.
func (s *Trips) CreateStop(ctx context.Context, tripID uuid.UUID, p StopParams) (Stop, error) {
	st, err := scanStop(s.pool.QueryRow(ctx, `
		INSERT INTO stops (trip_id, name, lat, lon, arrival_date, departure_date, notes, position)
		VALUES ($1, $2, $3, $4, $5, $6, $7,
		        (SELECT COALESCE(MAX(position) + 1, 0) FROM stops WHERE trip_id = $1))
		RETURNING `+stopColumns,
		tripID, p.Name, p.Lat, p.Lon, p.ArrivalDate, p.DepartureDate, p.Notes))
	if err == nil {
		s.touch(ctx, tripID)
	}
	return st, err
}

func (s *Trips) ListStops(ctx context.Context, tripID uuid.UUID) ([]Stop, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT `+stopColumns+` FROM stops WHERE trip_id = $1 ORDER BY position`, tripID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stops := []Stop{}
	for rows.Next() {
		st, err := scanStop(rows)
		if err != nil {
			return nil, err
		}
		stops = append(stops, st)
	}
	return stops, rows.Err()
}

func (s *Trips) StopByID(ctx context.Context, tripID, stopID uuid.UUID) (Stop, error) {
	return scanStop(s.pool.QueryRow(ctx,
		`SELECT `+stopColumns+` FROM stops WHERE id = $2 AND trip_id = $1`, tripID, stopID))
}

// UpdateStop replaces the stop's mutable fields. The trip ID is part of the
// WHERE clause so a stop can never be edited through another trip's URL.
func (s *Trips) UpdateStop(ctx context.Context, tripID, stopID uuid.UUID, p StopParams) (Stop, error) {
	st, err := scanStop(s.pool.QueryRow(ctx, `
		UPDATE stops
		SET name = $3, lat = $4, lon = $5, arrival_date = $6, departure_date = $7, notes = $8
		WHERE id = $2 AND trip_id = $1
		RETURNING `+stopColumns,
		tripID, stopID, p.Name, p.Lat, p.Lon, p.ArrivalDate, p.DepartureDate, p.Notes))
	if err == nil {
		s.touch(ctx, tripID)
	}
	return st, err
}

func (s *Trips) DeleteStop(ctx context.Context, tripID, stopID uuid.UUID) error {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM stops WHERE id = $2 AND trip_id = $1`, tripID, stopID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	s.touch(ctx, tripID)
	return nil
}

// ReorderStops sets the trip's stop ordering to exactly ids. It fails with
// ErrNotFound unless ids is a permutation of the trip's current stops.
func (s *Trips) ReorderStops(ctx context.Context, tripID uuid.UUID, ids []uuid.UUID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var count int
	if err := tx.QueryRow(ctx,
		`SELECT count(*) FROM stops WHERE trip_id = $1`, tripID).Scan(&count); err != nil {
		return err
	}
	if count != len(ids) {
		return ErrNotFound
	}

	// Offset first to avoid transient collisions with the target positions.
	if _, err := tx.Exec(ctx,
		`UPDATE stops SET position = position + $2 WHERE trip_id = $1`, tripID, len(ids)); err != nil {
		return err
	}
	for i, id := range ids {
		tag, err := tx.Exec(ctx,
			`UPDATE stops SET position = $3 WHERE id = $2 AND trip_id = $1`, tripID, id, i)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	s.touch(ctx, tripID)
	return nil
}
