package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/ctrl-research/waypoint/internal/auth"
	"github.com/ctrl-research/waypoint/internal/store"
)

type memberJSON struct {
	UserID      uuid.UUID `json:"userId"`
	Email       string    `json:"email"`
	DisplayName string    `json:"displayName"`
	AvatarURL   *string   `json:"avatarUrl"`
	Role        string    `json:"role"`
	AddedAt     time.Time `json:"addedAt"`
}

func (api *tripsAPI) listMembers(w http.ResponseWriter, r *http.Request) {
	trip, _, ok := api.tripAccess(w, r, "viewer")
	if !ok {
		return
	}
	members, err := api.trips.ListMembers(r.Context(), trip.ID)
	if err != nil {
		apiInternalError(w, "list members", err)
		return
	}
	out := make([]memberJSON, 0, len(members))
	for _, m := range members {
		out = append(out, memberJSON{
			UserID: m.UserID, Email: m.Email, DisplayName: m.DisplayName,
			AvatarURL: m.AvatarURL, Role: string(m.Role), AddedAt: m.CreatedAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"members": out})
}

// addMember invites by email: the address must belong to an existing user on
// this instance (there is no email delivery — see the PR notes). Repeating
// with a different role updates it.
func (api *tripsAPI) addMember(w http.ResponseWriter, r *http.Request) {
	trip, _, ok := api.tripAccess(w, r, "owner")
	if !ok {
		return
	}
	var req struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" {
		apiError(w, http.StatusBadRequest, "bad_request", "email is required")
		return
	}
	if !store.ValidTripRole(req.Role) {
		apiError(w, http.StatusBadRequest, "invalid", "role must be viewer or editor")
		return
	}
	target, err := api.users.ByEmail(r.Context(), req.Email)
	if errors.Is(err, store.ErrNotFound) {
		apiError(w, http.StatusNotFound, "no_such_user",
			"no account with that email on this instance — they need to sign in once first")
		return
	}
	if err != nil {
		apiInternalError(w, "lookup user", err)
		return
	}
	if target.ID == trip.OwnerID {
		apiError(w, http.StatusBadRequest, "invalid", "the owner already has full access")
		return
	}
	if err := api.trips.UpsertMember(r.Context(), trip.ID, target.ID, store.TripRole(req.Role)); err != nil {
		apiInternalError(w, "add member", err)
		return
	}
	writeJSON(w, http.StatusCreated, memberJSON{
		UserID: target.ID, Email: target.Email, DisplayName: target.DisplayName,
		AvatarURL: target.AvatarURL, Role: req.Role, AddedAt: time.Now(),
	})
}

// removeMember: the owner can remove anyone; a member can remove themselves
// (leave the trip).
func (api *tripsAPI) removeMember(w http.ResponseWriter, r *http.Request) {
	trip, role, ok := api.tripAccess(w, r, "viewer")
	if !ok {
		return
	}
	user, _ := auth.UserFrom(r.Context())
	targetID, err := uuid.Parse(r.PathValue("userID"))
	if err != nil {
		apiError(w, http.StatusNotFound, "not_found", "member not found")
		return
	}
	if role != "owner" && targetID != user.ID {
		apiError(w, http.StatusForbidden, "forbidden", "only the owner can remove other members")
		return
	}
	switch err := api.trips.RemoveMember(r.Context(), trip.ID, targetID); {
	case errors.Is(err, store.ErrNotFound):
		apiError(w, http.StatusNotFound, "not_found", "member not found")
	case err != nil:
		apiInternalError(w, "remove member", err)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}
