package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"time"

	"github.com/google/uuid"

	"github.com/ctrl-research/waypoint/internal/store"
)

var timeRe = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)

// itemRequest is used for POST and PATCH; nil pointers keep current values.
// day is required on create ("YYYY-MM-DD"); startTime is "HH:MM", empty
// string clears; costCents/currency must be set or cleared together
// (costCents -1 clears both).
type itemRequest struct {
	StopID    *uuid.UUID `json:"stopId"`
	ClearStop bool       `json:"clearStop"`
	Day       *string    `json:"day"`
	StartTime *string    `json:"startTime"`
	Title     *string    `json:"title"`
	Category  *string    `json:"category"`
	Notes     *string    `json:"notes"`
	CostCents *int64     `json:"costCents"`
	Currency  *string    `json:"currency"`
}

func (req itemRequest) merge(p *store.ItineraryItemParams) error {
	if req.ClearStop {
		p.StopID = nil
	} else if req.StopID != nil {
		p.StopID = req.StopID
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
		if *req.StartTime == "" {
			p.StartTime = nil
		} else if !timeRe.MatchString(*req.StartTime) {
			return errors.New("startTime must be HH:MM")
		} else {
			p.StartTime = req.StartTime
		}
	}
	if req.Title != nil {
		p.Title = *req.Title
	}
	if p.Title == "" {
		return errors.New("title is required")
	}
	if req.Category != nil {
		if !store.ValidItineraryCategory(*req.Category) {
			return errors.New("category must be activity, food, lodging, transport, or other")
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
	trip, ok := api.ownedTrip(w, r)
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
	item, err := api.trips.CreateItem(r.Context(), trip.ID, params)
	if err != nil {
		apiInternalError(w, "create item", err)
		return
	}
	writeJSON(w, http.StatusCreated, toItemJSON(item))
}

func (api *tripsAPI) updateItem(w http.ResponseWriter, r *http.Request) {
	trip, ok := api.ownedTrip(w, r)
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
		StopID: current.StopID, Day: current.Day, StartTime: current.StartTime,
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
	updated, err := api.trips.UpdateItem(r.Context(), trip.ID, itemID, params)
	if err != nil {
		apiInternalError(w, "update item", err)
		return
	}
	writeJSON(w, http.StatusOK, toItemJSON(updated))
}

func (api *tripsAPI) deleteItem(w http.ResponseWriter, r *http.Request) {
	trip, ok := api.ownedTrip(w, r)
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
