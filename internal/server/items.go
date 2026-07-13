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

func (api *tripsAPI) createItem(w http.ResponseWriter, r *http.Request) {
	trip, ok := api.editableTrip(w, r)
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
	item, err := api.trips.CreateItem(r.Context(), trip.ID, params)
	if err != nil {
		apiInternalError(w, "create item", err)
		return
	}
	writeJSON(w, http.StatusCreated, toItemJSON(item))
}

func (api *tripsAPI) updateItem(w http.ResponseWriter, r *http.Request) {
	trip, ok := api.editableTrip(w, r)
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
	}
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
	updated, err := api.trips.UpdateItem(r.Context(), trip.ID, itemID, params)
	if err != nil {
		apiInternalError(w, "update item", err)
		return
	}
	writeJSON(w, http.StatusOK, toItemJSON(updated))
}

// reorderItems replaces one day's ordering (used by the board's drag-drop).
func (api *tripsAPI) reorderItems(w http.ResponseWriter, r *http.Request) {
	trip, ok := api.editableTrip(w, r)
	if !ok {
		return
	}
	var req struct {
		Day string      `json:"day"`
		IDs []uuid.UUID `json:"ids"`
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
	switch err := api.trips.ReorderItems(r.Context(), trip.ID, day, req.IDs); {
	case errors.Is(err, store.ErrNotFound):
		apiError(w, http.StatusBadRequest, "invalid", "ids must be a permutation of that day's items")
	case err != nil:
		apiInternalError(w, "reorder items", err)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

func (api *tripsAPI) deleteItem(w http.ResponseWriter, r *http.Request) {
	trip, ok := api.editableTrip(w, r)
	if !ok {
		return
	}
	itemID, err := uuid.Parse(r.PathValue("itemID"))
	if err != nil {
		apiError(w, http.StatusNotFound, "not_found", "item not found")
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
