package store

import (
	"context"

	"github.com/google/uuid"

	"github.com/ctrl-research/waypoint/internal/store/sqlcgen"
)

// LocatedStop is a stop with coordinates, annotated with its trip.
type LocatedStop = sqlcgen.ListLocatedStopsForUserRow

// TravelLeg is a flight or train item with optional endpoint coordinates,
// times, and the trip owner's home as a fallback endpoint.
type TravelLeg = sqlcgen.ListTravelLegsForUserRow

// ListTravelLegs returns flight and train items across trips the user can see.
func (s *Trips) ListTravelLegs(ctx context.Context, userID uuid.UUID) ([]TravelLeg, error) {
	legs, err := s.q.ListTravelLegsForUser(ctx, userID)
	if legs == nil {
		legs = []TravelLeg{}
	}
	return legs, err
}

// ListLocatedStops returns every located stop across trips the user can see,
// in route order per trip.
func (s *Trips) ListLocatedStops(ctx context.Context, userID uuid.UUID) ([]LocatedStop, error) {
	stops, err := s.q.ListLocatedStopsForUser(ctx, userID)
	if stops == nil {
		stops = []LocatedStop{}
	}
	return stops, err
}
