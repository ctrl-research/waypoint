package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ctrl-research/waypoint/internal/store/sqlcgen"
)

type (
	Trip       = sqlcgen.Trip
	TripStatus = sqlcgen.TripStatus
)

const (
	TripPlanning  = sqlcgen.TripStatusPlanning
	TripActive    = sqlcgen.TripStatusActive
	TripCompleted = sqlcgen.TripStatusCompleted
)

func ValidTripStatus(s string) bool {
	switch TripStatus(s) {
	case TripPlanning, TripActive, TripCompleted:
		return true
	}
	return false
}

type TripParams struct {
	Title       string
	Description string
	Status      TripStatus
	StartDate   *time.Time
	EndDate     *time.Time
	CoverPhoto  *string
}

// Trips holds the pool (for reorder transactions) alongside the queries.
type Trips struct {
	pool *pgxpool.Pool
	q    *sqlcgen.Queries
}

func NewTrips(pool *pgxpool.Pool) *Trips {
	return &Trips{pool: pool, q: sqlcgen.New(pool)}
}

func (s *Trips) Create(ctx context.Context, ownerID uuid.UUID, p TripParams) (Trip, error) {
	t, err := s.q.CreateTrip(ctx, sqlcgen.CreateTripParams{
		OwnerID: ownerID, Title: p.Title, Description: p.Description, Status: p.Status,
		StartDate: p.StartDate, EndDate: p.EndDate, CoverPhoto: p.CoverPhoto,
	})
	return t, translate(err)
}

func (s *Trips) ByID(ctx context.Context, id uuid.UUID) (Trip, error) {
	t, err := s.q.TripByID(ctx, id)
	return t, translate(err)
}

func (s *Trips) ListByOwner(ctx context.Context, ownerID uuid.UUID) ([]Trip, error) {
	trips, err := s.q.ListTripsByOwner(ctx, ownerID)
	if trips == nil {
		trips = []Trip{}
	}
	return trips, err
}

// Update replaces all mutable fields; the handler merges PATCH semantics.
func (s *Trips) Update(ctx context.Context, id uuid.UUID, p TripParams) (Trip, error) {
	t, err := s.q.UpdateTrip(ctx, sqlcgen.UpdateTripParams{
		ID: id, Title: p.Title, Description: p.Description, Status: p.Status,
		StartDate: p.StartDate, EndDate: p.EndDate, CoverPhoto: p.CoverPhoto,
	})
	return t, translate(err)
}

func (s *Trips) Delete(ctx context.Context, id uuid.UUID) error {
	n, err := s.q.DeleteTrip(ctx, id)
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// touch bumps the trip's updated_at when a child row changes.
func (s *Trips) touch(ctx context.Context, tripID uuid.UUID) {
	_ = s.q.TouchTrip(ctx, tripID)
}
