package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"time"

	"github.com/google/uuid"

	"github.com/ctrl-research/waypoint/internal/auth"
	"github.com/ctrl-research/waypoint/internal/store"
)

var timeRe = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)

// itemRequest is used for POST and PATCH; nil pointers keep current values.
// day is required on create ("YYYY-MM-DD"); startTime is "HH:MM", empty
// string clears; costCents/currency must be set or cleared together
// (costCents -1 clears both).
type itemRequest struct {
	StopID            *uuid.UUID `json:"stopId"`
	ClearStop         bool       `json:"clearStop"`
	DestinationStopID *uuid.UUID `json:"destinationStopId"`
	ClearDestination  bool       `json:"clearDestination"`
	OriginHomeID      *uuid.UUID `json:"originHomeId"`
	ClearOriginHome   bool       `json:"clearOriginHome"`
	DestinationHomeID *uuid.UUID `json:"destinationHomeId"`
	ClearDestHome     bool       `json:"clearDestinationHome"`
	Day               *string    `json:"day"`
	StartTime         *string    `json:"startTime"`
	EndTime           *string    `json:"endTime"`
	Title             *string    `json:"title"`
	Category          *string    `json:"category"`
	Notes             *string    `json:"notes"`
	CostCents         *int64     `json:"costCents"`
	Currency          *string    `json:"currency"`
	Address           *string    `json:"address"`
	Lat               *float64   `json:"lat"`
	Lon               *float64   `json:"lon"`
	ClearLatLon       bool       `json:"clearLatLon"`
	LayerID           *uuid.UUID `json:"layerId"`
}

func (req itemRequest) merge(p *store.ItineraryItemParams) error {
	if req.ClearStop {
		p.StopID = nil
	} else if req.StopID != nil {
		p.StopID = req.StopID
	}
	if req.ClearDestination {
		p.DestinationStopID = nil
	} else if req.DestinationStopID != nil {
		p.DestinationStopID = req.DestinationStopID
	}
	if req.ClearOriginHome {
		p.OriginHomeID = nil
	} else if req.OriginHomeID != nil {
		p.OriginHomeID = req.OriginHomeID
	}
	if req.ClearDestHome {
		p.DestinationHomeID = nil
	} else if req.DestinationHomeID != nil {
		p.DestinationHomeID = req.DestinationHomeID
	}
	// A home and a stop cannot both anchor the same end of a leg.
	if p.OriginHomeID != nil && p.StopID != nil {
		return errors.New("originHomeId and stopId are mutually exclusive")
	}
	if p.DestinationHomeID != nil && p.DestinationStopID != nil {
		return errors.New("destinationHomeId and destinationStopId are mutually exclusive")
	}
	if req.Day != nil {
		d, err := time.Parse(dateFormat, *req.Day)
		if err != nil {
			return errors.New("day must be YYYY-MM-DD")
		}
		p.Day = d
	}
	if p.Day.IsZero() {
		return errors.New("day is required")
	}
	if req.StartTime != nil {
		if *req.StartTime != "" && !timeRe.MatchString(*req.StartTime) {
			return errors.New("startTime must be HH:MM")
		}
		p.StartTime = *req.StartTime
	}
	if req.EndTime != nil {
		if *req.EndTime != "" && !timeRe.MatchString(*req.EndTime) {
			return errors.New("endTime must be HH:MM")
		}
		p.EndTime = *req.EndTime
	}
	if req.Title != nil {
		p.Title = *req.Title
	}
	if p.Title == "" {
		return errors.New("title is required")
	}
	if req.Category != nil {
		if !store.ValidItineraryCategory(*req.Category) {
			return errors.New("category must be activity, food, lodging, transport, flight, train, or other")
		}
		p.Category = store.ItineraryCategory(*req.Category)
	}
	if p.Category == "" {
		p.Category = store.CategoryOther
	}
	if req.Notes != nil {
		p.Notes = *req.Notes
	}
	if req.Address != nil {
		p.Address = *req.Address
	}
	switch {
	case req.ClearLatLon:
		p.Lat, p.Lon = nil, nil
	case req.Lat != nil || req.Lon != nil:
		if req.Lat == nil || req.Lon == nil {
			return errors.New("lat and lon must be provided together")
		}
		if *req.Lat < -90 || *req.Lat > 90 || *req.Lon < -180 || *req.Lon > 180 {
			return errors.New("lat/lon out of range")
		}
		p.Lat, p.Lon = req.Lat, req.Lon
	}
	if req.CostCents != nil && *req.CostCents == -1 {
		p.CostCents, p.Currency = nil, nil
	} else if req.CostCents != nil || req.Currency != nil {
		if req.CostCents != nil {
			p.CostCents = req.CostCents
		}
		if req.Currency != nil {
			p.Currency = req.Currency
		}
		if (p.CostCents == nil) != (p.Currency == nil) {
			return errors.New("costCents and currency must be provided together")
		}
		if p.CostCents != nil && *p.CostCents < 0 {
			return errors.New("costCents must be non-negative")
		}
		if p.Currency != nil && len(*p.Currency) != 3 {
			return errors.New("currency must be a 3-letter ISO code")
		}
	}
	return nil
}

// homesBelongToUser verifies referenced homes are the requester's own.
func (api *tripsAPI) homesBelongToUser(r *http.Request, ids ...*uuid.UUID) (bool, error) {
	user, _ := auth.UserFrom(r.Context())
	for _, id := range ids {
		if id == nil {
			continue
		}
		if _, err := api.trips.HomeByID(r.Context(), user.ID, *id); errors.Is(err, store.ErrNotFound) {
			return false, nil
		} else if err != nil {
			return false, err
		}
	}
	return true, nil
}

// stopBelongsToTrip guards against attaching an item to another trip's stop.
func (api *tripsAPI) stopBelongsToTrip(r *http.Request, tripID uuid.UUID, stopID *uuid.UUID) (bool, error) {
	if stopID == nil {
		return true, nil
	}
	_, err := api.trips.StopByID(r.Context(), tripID, *stopID)
	if errors.Is(err, store.ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}

// itemEditable resolves the item's layer and applies layerEditable.
func (api *tripsAPI) itemEditable(r *http.Request, role string, item store.ItineraryItem, userID uuid.UUID) (bool, error) {
	layer, err := api.trips.LayerByID(r.Context(), item.TripID, item.LayerID)
	if err != nil {
		return false, err
	}
	return layerEditable(role, layer, userID), nil
}

func (api *tripsAPI) createItem(w http.ResponseWriter, r *http.Request) {
	// Viewers get in too: they may add items to their own proposal layer.
	trip, role, ok := api.tripAccess(w, r, "viewer")
	if !ok {
		return
	}
	var req itemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apiError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}
	var params store.ItineraryItemParams
	if err := req.merge(&params); err != nil {
		apiError(w, http.StatusBadRequest, "invalid", err.Error())
		return
	}
	if ok, err := api.stopBelongsToTrip(r, trip.ID, params.StopID); err != nil {
		apiInternalError(w, "check stop", err)
		return
	} else if !ok {
		apiError(w, http.StatusBadRequest, "invalid", "stopId does not belong to this trip")
		return
	}
	if ok, err := api.stopBelongsToTrip(r, trip.ID, params.DestinationStopID); err != nil {
		apiInternalError(w, "check destination", err)
		return
	} else if !ok {
		apiError(w, http.StatusBadRequest, "invalid", "destinationStopId does not belong to this trip")
		return
	}
	if ok, err := api.homesBelongToUser(r, params.OriginHomeID, params.DestinationHomeID); err != nil {
		apiInternalError(w, "check homes", err)
		return
	} else if !ok {
		apiError(w, http.StatusBadRequest, "invalid", "home does not belong to you")
		return
	}
	// Items land on an explicit layer or the trip's Final layer (#73).
	var layer store.ItineraryLayer
	if req.LayerID != nil {
		var err error
		if layer, err = api.trips.LayerByID(r.Context(), trip.ID, *req.LayerID); err != nil {
			apiError(w, http.StatusBadRequest, "invalid", "layerId does not belong to this trip")
			return
		}
	} else {
		var err error
		if layer, err = api.trips.EnsureFinalLayer(r.Context(), trip.ID); err != nil {
			apiInternalError(w, "ensure final layer", err)
			return
		}
	}
	user, _ := auth.UserFrom(r.Context())
	if !layerEditable(role, layer, user.ID) {
		apiError(w, http.StatusForbidden, "forbidden", "you can only add items to your own layer")
		return
	}
	params.LayerID = layer.ID
	item, err := api.trips.CreateItem(r.Context(), trip.ID, params)
	if err != nil {
		apiInternalError(w, "create item", err)
		return
	}
	writeJSON(w, http.StatusCreated, toItemJSON(item))
}

func (api *tripsAPI) updateItem(w http.ResponseWriter, r *http.Request) {
	trip, role, ok := api.tripAccess(w, r, "viewer")
	if !ok {
		return
	}
	itemID, err := uuid.Parse(r.PathValue("itemID"))
	if err != nil {
		apiError(w, http.StatusNotFound, "not_found", "item not found")
		return
	}
	current, err := api.trips.ItemByID(r.Context(), trip.ID, itemID)
	if errors.Is(err, store.ErrNotFound) {
		apiError(w, http.StatusNotFound, "not_found", "item not found")
		return
	}
	if err != nil {
		apiInternalError(w, "load item", err)
		return
	}
	user, _ := auth.UserFrom(r.Context())
	if ok, err := api.itemEditable(r, role, current, user.ID); err != nil {
		apiInternalError(w, "load item layer", err)
		return
	} else if !ok {
		apiError(w, http.StatusForbidden, "forbidden", "you can only edit items on your own layer")
		return
	}

	var req itemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apiError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}
	params := store.ItineraryItemParams{
		StopID: current.StopID, DestinationStopID: current.DestinationStopID,
		OriginHomeID: current.OriginHomeID, DestinationHomeID: current.DestinationHomeID,
		Day: current.Day, StartTime: current.StartTime, EndTime: current.EndTime,
		Title: current.Title, Category: current.Category, Notes: current.Notes,
		CostCents: current.CostCents, Currency: current.Currency,
		Address: current.Address, Lat: current.Lat, Lon: current.Lon,
		LayerID: current.LayerID,
	}
	if err := req.merge(&params); err != nil {
		apiError(w, http.StatusBadRequest, "invalid", err.Error())
		return
	}
	// Moving between layers is promotion/demotion (#73): the caller must be
	// allowed to write the target layer too (Final needs editor+).
	if req.LayerID != nil && *req.LayerID != current.LayerID {
		target, err := api.trips.LayerByID(r.Context(), trip.ID, *req.LayerID)
		if err != nil {
			apiError(w, http.StatusBadRequest, "invalid", "layerId does not belong to this trip")
			return
		}
		if !layerEditable(role, target, user.ID) {
			apiError(w, http.StatusForbidden, "forbidden", "you cannot move items onto that layer")
			return
		}
		params.LayerID = target.ID
	}
	if ok, err := api.stopBelongsToTrip(r, trip.ID, params.StopID); err != nil {
		apiInternalError(w, "check stop", err)
		return
	} else if !ok {
		apiError(w, http.StatusBadRequest, "invalid", "stopId does not belong to this trip")
		return
	}
	if ok, err := api.stopBelongsToTrip(r, trip.ID, params.DestinationStopID); err != nil {
		apiInternalError(w, "check destination", err)
		return
	} else if !ok {
		apiError(w, http.StatusBadRequest, "invalid", "destinationStopId does not belong to this trip")
		return
	}
	if ok, err := api.homesBelongToUser(r, params.OriginHomeID, params.DestinationHomeID); err != nil {
		apiInternalError(w, "check homes", err)
		return
	} else if !ok {
		apiError(w, http.StatusBadRequest, "invalid", "home does not belong to you")
		return
	}
	updated, err := api.trips.UpdateItem(r.Context(), trip.ID, itemID, params)
	if err != nil {
		apiInternalError(w, "update item", err)
		return
	}
	writeJSON(w, http.StatusOK, toItemJSON(updated))
}

// reorderItems replaces one day's ordering on one layer (the board's
// drag-drop). A missing layerId means the Final layer.
func (api *tripsAPI) reorderItems(w http.ResponseWriter, r *http.Request) {
	trip, role, ok := api.tripAccess(w, r, "viewer")
	if !ok {
		return
	}
	var req struct {
		Day     string      `json:"day"`
		LayerID *uuid.UUID  `json:"layerId"`
		IDs     []uuid.UUID `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apiError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}
	day, err := time.Parse(dateFormat, req.Day)
	if err != nil {
		apiError(w, http.StatusBadRequest, "invalid", "day must be YYYY-MM-DD")
		return
	}
	var layer store.ItineraryLayer
	if req.LayerID != nil {
		if layer, err = api.trips.LayerByID(r.Context(), trip.ID, *req.LayerID); err != nil {
			apiError(w, http.StatusBadRequest, "invalid", "layerId does not belong to this trip")
			return
		}
	} else {
		if layer, err = api.trips.EnsureFinalLayer(r.Context(), trip.ID); err != nil {
			apiInternalError(w, "ensure final layer", err)
			return
		}
	}
	user, _ := auth.UserFrom(r.Context())
	if !layerEditable(role, layer, user.ID) {
		apiError(w, http.StatusForbidden, "forbidden", "you can only reorder your own layer")
		return
	}
	switch err := api.trips.ReorderItems(r.Context(), trip.ID, day, layer.ID, req.IDs); {
	case errors.Is(err, store.ErrNotFound):
		apiError(w, http.StatusBadRequest, "invalid", "ids must be a permutation of that day's items")
	case err != nil:
		apiInternalError(w, "reorder items", err)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

func (api *tripsAPI) deleteItem(w http.ResponseWriter, r *http.Request) {
	trip, role, ok := api.tripAccess(w, r, "viewer")
	if !ok {
		return
	}
	itemID, err := uuid.Parse(r.PathValue("itemID"))
	if err != nil {
		apiError(w, http.StatusNotFound, "not_found", "item not found")
		return
	}
	item, err := api.trips.ItemByID(r.Context(), trip.ID, itemID)
	if errors.Is(err, store.ErrNotFound) {
		apiError(w, http.StatusNotFound, "not_found", "item not found")
		return
	}
	if err != nil {
		apiInternalError(w, "load item", err)
		return
	}
	user, _ := auth.UserFrom(r.Context())
	if ok, err := api.itemEditable(r, role, item, user.ID); err != nil {
		apiInternalError(w, "load item layer", err)
		return
	} else if !ok {
		apiError(w, http.StatusForbidden, "forbidden", "you can only delete items on your own layer")
		return
	}
	switch err := api.trips.DeleteItem(r.Context(), trip.ID, itemID); {
	case errors.Is(err, store.ErrNotFound):
		apiError(w, http.StatusNotFound, "not_found", "item not found")
	case err != nil:
		apiInternalError(w, "delete item", err)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}
