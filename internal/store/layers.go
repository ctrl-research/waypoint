package store

import (
	"context"

	"github.com/google/uuid"

	"github.com/ctrl-research/waypoint/internal/store/sqlcgen"
)

// ItineraryLayer groups itinerary items for the collaborative editor (#73).
// OwnerID nil marks the trip's default Main layer; members create any
// number of named layers, and the itinerary is the merge of visible ones.
type ItineraryLayer = sqlcgen.ItineraryLayer

// EnsureMainLayer returns the trip's Main layer, creating it on first use.
func (s *Trips) EnsureMainLayer(ctx context.Context, tripID uuid.UUID) (ItineraryLayer, error) {
	l, err := s.q.EnsureMainLayer(ctx, tripID)
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

// CreateLayer adds a member-owned named layer; members can hold several.
func (s *Trips) CreateLayer(ctx context.Context, tripID, ownerID uuid.UUID, name, color string) (ItineraryLayer, error) {
	l, err := s.q.CreateLayer(ctx, sqlcgen.CreateLayerParams{
		TripID: tripID, OwnerID: &ownerID, Name: name, Color: color,
	})
	return l, translate(err)
}

func (s *Trips) UpdateLayer(ctx context.Context, tripID, layerID uuid.UUID, name, color string, visible bool) (ItineraryLayer, error) {
	l, err := s.q.UpdateLayer(ctx, sqlcgen.UpdateLayerParams{
		TripID: tripID, ID: layerID, Name: name, Color: color, Visible: visible,
	})
	if err == nil {
		s.touch(ctx, tripID)
	}
	return l, translate(err)
}

// DeleteProposalLayer removes a member's layer and its items. The Main
// layer is excluded in SQL, so targeting it reports ErrNotFound.
func (s *Trips) DeleteProposalLayer(ctx context.Context, tripID, layerID uuid.UUID) error {
	n, err := s.q.DeleteProposalLayer(ctx, sqlcgen.DeleteProposalLayerParams{TripID: tripID, ID: layerID})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	s.touch(ctx, tripID)
	return nil
}
