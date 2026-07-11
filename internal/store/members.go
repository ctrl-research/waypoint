package store

import (
	"context"

	"github.com/google/uuid"

	"github.com/ctrl-research/waypoint/internal/store/sqlcgen"
)

type (
	TripRole = sqlcgen.TripRole
	// Member is a trip membership joined with the user's identity.
	Member = sqlcgen.ListMembersRow
	// AccessibleTrip is a trip annotated with the requesting user's role.
	AccessibleTrip = sqlcgen.ListAccessibleTripsRow
)

const (
	RoleViewer = sqlcgen.TripRoleViewer
	RoleEditor = sqlcgen.TripRoleEditor
)

func ValidTripRole(r string) bool {
	return TripRole(r) == RoleViewer || TripRole(r) == RoleEditor
}

// WithRole loads a trip and the user's role on it: "owner", "editor",
// "viewer", or "" when the user has no access.
func (s *Trips) WithRole(ctx context.Context, tripID, userID uuid.UUID) (Trip, string, error) {
	row, err := s.q.TripWithRole(ctx, sqlcgen.TripWithRoleParams{ID: tripID, OwnerID: userID})
	return row.Trip, row.Role, translate(err)
}

// ListAccessible returns trips the user owns or is a member of.
func (s *Trips) ListAccessible(ctx context.Context, userID uuid.UUID) ([]AccessibleTrip, error) {
	trips, err := s.q.ListAccessibleTrips(ctx, userID)
	if trips == nil {
		trips = []AccessibleTrip{}
	}
	return trips, err
}

func (s *Trips) UpsertMember(ctx context.Context, tripID, userID uuid.UUID, role TripRole) error {
	_, err := s.q.UpsertMember(ctx, sqlcgen.UpsertMemberParams{TripID: tripID, UserID: userID, Role: role})
	return err
}

func (s *Trips) ListMembers(ctx context.Context, tripID uuid.UUID) ([]Member, error) {
	members, err := s.q.ListMembers(ctx, tripID)
	if members == nil {
		members = []Member{}
	}
	return members, err
}

func (s *Trips) RemoveMember(ctx context.Context, tripID, userID uuid.UUID) error {
	n, err := s.q.RemoveMember(ctx, sqlcgen.RemoveMemberParams{TripID: tripID, UserID: userID})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
