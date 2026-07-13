package store

import (
	"context"

	"github.com/google/uuid"

	"github.com/ctrl-research/waypoint/internal/store/sqlcgen"
)

// LocatedStop is a stop with coordinates, annotated with its trip.
type LocatedStop = sqlcgen.ListLocatedStopsForUserRow

// ListLocatedStops returns every located stop across trips the user can see,
// in route order per trip.
func (s *Trips) ListLocatedStops(ctx context.Context, userID uuid.UUID) ([]LocatedStop, error) {
	stops, err := s.q.ListLocatedStopsForUser(ctx, userID)
	if stops == nil {
		stops = []LocatedStop{}
	}
	return stops, err
}
