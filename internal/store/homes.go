package store

import (
	"context"

	"github.com/google/uuid"

	"github.com/ctrl-research/waypoint/internal/store/sqlcgen"
)

type (
	Home = sqlcgen.Home
	// TripHome is a home referenced by a trip's legs, for UI labels.
	TripHome = sqlcgen.ListTripHomesRow
)

// Homes are per-user saved locations that flight/train legs can use as
// endpoints. All operations are scoped to the owning user.
func (s *Trips) CreateHome(ctx context.Context, userID uuid.UUID, name string, lat, lon float64) (Home, error) {
	h, err := s.q.CreateHome(ctx, sqlcgen.CreateHomeParams{UserID: userID, Name: name, Lat: lat, Lon: lon})
	return h, translate(err)
}

func (s *Trips) ListHomes(ctx context.Context, userID uuid.UUID) ([]Home, error) {
	homes, err := s.q.ListHomes(ctx, userID)
	if homes == nil {
		homes = []Home{}
	}
	return homes, err
}

func (s *Trips) HomeByID(ctx context.Context, userID, homeID uuid.UUID) (Home, error) {
	h, err := s.q.HomeByID(ctx, sqlcgen.HomeByIDParams{UserID: userID, ID: homeID})
	return h, translate(err)
}

func (s *Trips) DeleteHome(ctx context.Context, userID, homeID uuid.UUID) error {
	n, err := s.q.DeleteHome(ctx, sqlcgen.DeleteHomeParams{UserID: userID, ID: homeID})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ListTripHomes returns homes referenced by a trip's legs (for labels).
func (s *Trips) ListTripHomes(ctx context.Context, tripID uuid.UUID) ([]TripHome, error) {
	homes, err := s.q.ListTripHomes(ctx, tripID)
	if homes == nil {
		homes = []TripHome{}
	}
	return homes, err
}
