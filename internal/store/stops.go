package store

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/ctrl-research/waypoint/internal/store/sqlcgen"
)

type Stop = sqlcgen.Stop

type StopParams struct {
	Name          string
	Lat           *float64
	Lon           *float64
	ArrivalDate   *time.Time
	DepartureDate *time.Time
	Notes         string
}

// CreateStop appends the stop at the end of the trip's ordering.
func (s *Trips) CreateStop(ctx context.Context, tripID uuid.UUID, p StopParams) (Stop, error) {
	st, err := s.q.CreateStop(ctx, sqlcgen.CreateStopParams{
		TripID: tripID, Name: p.Name, Lat: p.Lat, Lon: p.Lon,
		ArrivalDate: p.ArrivalDate, DepartureDate: p.DepartureDate, Notes: p.Notes,
	})
	if err == nil {
		s.touch(ctx, tripID)
	}
	return st, translate(err)
}

func (s *Trips) ListStops(ctx context.Context, tripID uuid.UUID) ([]Stop, error) {
	stops, err := s.q.ListStops(ctx, tripID)
	if stops == nil {
		stops = []Stop{}
	}
	return stops, err
}

func (s *Trips) StopByID(ctx context.Context, tripID, stopID uuid.UUID) (Stop, error) {
	st, err := s.q.StopByID(ctx, sqlcgen.StopByIDParams{TripID: tripID, ID: stopID})
	return st, translate(err)
}

// UpdateStop replaces the stop's mutable fields. The trip ID is part of the
// WHERE clause so a stop can never be edited through another trip's URL.
func (s *Trips) UpdateStop(ctx context.Context, tripID, stopID uuid.UUID, p StopParams) (Stop, error) {
	st, err := s.q.UpdateStop(ctx, sqlcgen.UpdateStopParams{
		TripID: tripID, ID: stopID, Name: p.Name, Lat: p.Lat, Lon: p.Lon,
		ArrivalDate: p.ArrivalDate, DepartureDate: p.DepartureDate, Notes: p.Notes,
	})
	if err == nil {
		s.touch(ctx, tripID)
	}
	return st, translate(err)
}

func (s *Trips) DeleteStop(ctx context.Context, tripID, stopID uuid.UUID) error {
	n, err := s.q.DeleteStop(ctx, sqlcgen.DeleteStopParams{TripID: tripID, ID: stopID})
	if err != nil {
		return err
	}
	if n == 0 {
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
	q := s.q.WithTx(tx)

	count, err := q.CountStops(ctx, tripID)
	if err != nil {
		return err
	}
	if count != int64(len(ids)) {
		return ErrNotFound
	}

	// Offset first to avoid transient collisions with the target positions.
	if err := q.OffsetStopPositions(ctx, sqlcgen.OffsetStopPositionsParams{
		TripID: tripID, Position: int32(len(ids)),
	}); err != nil {
		return err
	}
	for i, id := range ids {
		n, err := q.SetStopPosition(ctx, sqlcgen.SetStopPositionParams{
			TripID: tripID, ID: id, Position: int32(i),
		})
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrNotFound
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	s.touch(ctx, tripID)
	return nil
}
