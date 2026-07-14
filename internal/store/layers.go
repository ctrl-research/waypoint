package store

import (
	"context"

	"github.com/google/uuid"

	"github.com/ctrl-research/waypoint/internal/store/sqlcgen"
)

// ItineraryLayer groups itinerary items for the collaborative editor (#73).
// OwnerID nil marks the trip's single Final layer — the published plan.
type ItineraryLayer = sqlcgen.ItineraryLayer

// EnsureFinalLayer returns the trip's Final layer, creating it on first use.
func (s *Trips) EnsureFinalLayer(ctx context.Context, tripID uuid.UUID) (ItineraryLayer, error) {
	l, err := s.q.EnsureFinalLayer(ctx, tripID)
	return l, translate(err)
}

func (s *Trips) ListLayers(ctx context.Context, tripID uuid.UUID) ([]ItineraryLayer, error) {
	layers, err := s.q.ListLayers(ctx, tripID)
	if layers == nil {
		layers = []ItineraryLayer{}
	}
	return layers, err
}

func (s *Trips) LayerByID(ctx context.Context, tripID, layerID uuid.UUID) (ItineraryLayer, error) {
	l, err := s.q.LayerByID(ctx, sqlcgen.LayerByIDParams{TripID: tripID, ID: layerID})
	return l, translate(err)
}
