package store

import (
	"context"

	"github.com/google/uuid"

	"github.com/ctrl-research/waypoint/internal/store/sqlcgen"
)

// LocatedStop is a stop with coordinates, annotated with its trip.
type LocatedStop = sqlcgen.ListLocatedStopsForUserRow

// FlightLeg is a flight item with optional endpoint coordinates and times.
type FlightLeg = sqlcgen.ListFlightLegsForUserRow

// ListFlightLegs returns flight items across trips the user can see.
func (s *Trips) ListFlightLegs(ctx context.Context, userID uuid.UUID) ([]FlightLeg, error) {
	legs, err := s.q.ListFlightLegsForUser(ctx, userID)
	if legs == nil {
		legs = []FlightLeg{}
	}
	return legs, err
}

// CountDistinctStopNames counts distinct stop names (located or not) across
// trips the user can see.
func (s *Trips) CountDistinctStopNames(ctx context.Context, userID uuid.UUID) (int64, error) {
	return s.q.CountDistinctStopNamesForUser(ctx, userID)
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
