package store

import (
	"context"

	"github.com/google/uuid"

	"github.com/ctrl-research/waypoint/internal/store/sqlcgen"
)

type ShareLink = sqlcgen.ShareLink

func (s *Trips) CreateShareLink(ctx context.Context, tripID uuid.UUID, token string) (ShareLink, error) {
	link, err := s.q.CreateShareLink(ctx, sqlcgen.CreateShareLinkParams{TripID: tripID, Token: token})
	return link, translate(err)
}

func (s *Trips) ListShareLinks(ctx context.Context, tripID uuid.UUID) ([]ShareLink, error) {
	links, err := s.q.ListShareLinks(ctx, tripID)
	if links == nil {
		links = []ShareLink{}
	}
	return links, err
}

func (s *Trips) DeleteShareLink(ctx context.Context, tripID, linkID uuid.UUID) error {
	n, err := s.q.DeleteShareLink(ctx, sqlcgen.DeleteShareLinkParams{TripID: tripID, ID: linkID})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// TripByShareToken resolves a public share token to its trip.
func (s *Trips) TripByShareToken(ctx context.Context, token string) (Trip, error) {
	t, err := s.q.TripByShareToken(ctx, token)
	return t, translate(err)
}
