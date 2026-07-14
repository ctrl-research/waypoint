package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"

	"github.com/google/uuid"

	"github.com/ctrl-research/waypoint/internal/auth"
	"github.com/ctrl-research/waypoint/internal/store"
)

var hexColorRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// proposalPalette colors new member layers in fixed order (dataviz
// categorical rules); the Main layer keeps the default #2a78d6 blue.
var proposalPalette = []string{"#d97706", "#059669", "#7c3aed", "#db2777", "#0891b2", "#65a30d"}

func layerJSON(l store.ItineraryLayer) map[string]any {
	return map[string]any{"id": l.ID, "name": l.Name, "color": l.Color, "ownerId": l.OwnerID, "visible": l.Visible}
}

// layerEditable reports whether the user may change a layer's items:
// editors and up touch any layer; every member may work their own proposal.
func layerEditable(role string, layer store.ItineraryLayer, userID uuid.UUID) bool {
	if roleRank(role) >= roleRank("editor") {
		return true
	}
	return layer.OwnerID != nil && *layer.OwnerID == userID
}

// createLayer adds a named member layer — any member, any number of
// layers, organized however they like (#73 follow-up).
func (api *tripsAPI) createLayer(w http.ResponseWriter, r *http.Request) {
	trip, _, ok := api.tripAccess(w, r, "viewer")
	if !ok {
		return
	}
	user, _ := auth.UserFrom(r.Context())

	var req struct {
		Name  string `json:"name"`
		Color string `json:"color"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apiError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		apiError(w, http.StatusBadRequest, "invalid", "name is required")
		return
	}
	if req.Color == "" {
		layers, err := api.trips.ListLayers(r.Context(), trip.ID)
		if err != nil {
			apiInternalError(w, "list layers", err)
			return
		}
		owned := 0
		for _, l := range layers {
			if l.OwnerID != nil {
				owned++
			}
		}
		req.Color = proposalPalette[owned%len(proposalPalette)]
	} else if !hexColorRe.MatchString(req.Color) {
		apiError(w, http.StatusBadRequest, "invalid", "color must be #rrggbb")
		return
	}

	layer, err := api.trips.CreateLayer(r.Context(), trip.ID, user.ID, req.Name, req.Color)
	if err != nil {
		apiInternalError(w, "create layer", err)
		return
	}
	writeJSON(w, http.StatusCreated, layerJSON(layer))
}

// layerFromPath resolves {layerID} within the trip; missing → 404.
func (api *tripsAPI) layerFromPath(w http.ResponseWriter, r *http.Request, tripID uuid.UUID) (store.ItineraryLayer, bool) {
	layerID, err := uuid.Parse(r.PathValue("layerID"))
	if err != nil {
		apiError(w, http.StatusNotFound, "not_found", "layer not found")
		return store.ItineraryLayer{}, false
	}
	layer, err := api.trips.LayerByID(r.Context(), tripID, layerID)
	if errors.Is(err, store.ErrNotFound) {
		apiError(w, http.StatusNotFound, "not_found", "layer not found")
		return store.ItineraryLayer{}, false
	}
	if err != nil {
		apiInternalError(w, "load layer", err)
		return store.ItineraryLayer{}, false
	}
	return layer, true
}

// layerManageable guards layer metadata changes: the Main layer needs
// editor+, a member layer belongs to its owner (the trip owner can moderate).
func layerManageable(role string, layer store.ItineraryLayer, userID uuid.UUID) bool {
	if layer.OwnerID == nil {
		return roleRank(role) >= roleRank("editor")
	}
	return *layer.OwnerID == userID || role == "owner"
}

func (api *tripsAPI) updateLayer(w http.ResponseWriter, r *http.Request) {
	trip, role, ok := api.tripAccess(w, r, "viewer")
	if !ok {
		return
	}
	layer, ok := api.layerFromPath(w, r, trip.ID)
	if !ok {
		return
	}
	user, _ := auth.UserFrom(r.Context())

	var req struct {
		Name    *string `json:"name"`
		Color   *string `json:"color"`
		Visible *bool   `json:"visible"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apiError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}
	name, color, visible := layer.Name, layer.Color, layer.Visible
	if req.Name != nil {
		if *req.Name == "" {
			apiError(w, http.StatusBadRequest, "invalid", "name cannot be empty")
			return
		}
		name = *req.Name
	}
	if req.Color != nil {
		if !hexColorRe.MatchString(*req.Color) {
			apiError(w, http.StatusBadRequest, "invalid", "color must be #rrggbb")
			return
		}
		color = *req.Color
	}
	if req.Visible != nil {
		visible = *req.Visible
	}
	// Rename/recolor is the owner's call; visibility shapes the shared
	// itinerary, so editors may curate any layer (and members their own).
	if (req.Name != nil || req.Color != nil) && !layerManageable(role, layer, user.ID) {
		apiError(w, http.StatusForbidden, "forbidden", "you cannot manage this layer")
		return
	}
	if req.Visible != nil && !layerEditable(role, layer, user.ID) {
		apiError(w, http.StatusForbidden, "forbidden", "you cannot change this layer's visibility")
		return
	}
	updated, err := api.trips.UpdateLayer(r.Context(), trip.ID, layer.ID, name, color, visible)
	if err != nil {
		apiInternalError(w, "update layer", err)
		return
	}
	writeJSON(w, http.StatusOK, layerJSON(updated))
}

func (api *tripsAPI) deleteLayer(w http.ResponseWriter, r *http.Request) {
	trip, role, ok := api.tripAccess(w, r, "viewer")
	if !ok {
		return
	}
	layer, ok := api.layerFromPath(w, r, trip.ID)
	if !ok {
		return
	}
	if layer.OwnerID == nil {
		apiError(w, http.StatusBadRequest, "invalid", "the Main layer cannot be deleted")
		return
	}
	user, _ := auth.UserFrom(r.Context())
	if !layerManageable(role, layer, user.ID) {
		apiError(w, http.StatusForbidden, "forbidden", "you cannot manage this layer")
		return
	}
	if err := api.trips.DeleteProposalLayer(r.Context(), trip.ID, layer.ID); err != nil {
		apiInternalError(w, "delete layer", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
