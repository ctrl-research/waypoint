package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/ctrl-research/waypoint/internal/store"
)

// stopRequest is used for POST and PATCH; nil pointers keep current values.
// lat/lon must be set or cleared together. Dates use "YYYY-MM-DD"; empty
// string clears.
type stopRequest struct {
	Name          *string  `json:"name"`
	Lat           *float64 `json:"lat"`
	Lon           *float64 `json:"lon"`
	ClearLatLon   bool     `json:"clearLatLon"`
	ArrivalDate   *string  `json:"arrivalDate"`
	DepartureDate *string  `json:"departureDate"`
	Notes         *string  `json:"notes"`
}

func (req stopRequest) merge(p *store.StopParams) error {
	if req.Name != nil {
		p.Name = *req.Name
	}
	if p.Name == "" {
		return errors.New("name is required")
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
	var err error
	if p.ArrivalDate, err = mergeDate(req.ArrivalDate, p.ArrivalDate, "arrivalDate"); err != nil {
		return err
	}
	if p.DepartureDate, err = mergeDate(req.DepartureDate, p.DepartureDate, "departureDate"); err != nil {
		return err
	}
	if req.Notes != nil {
		p.Notes = *req.Notes
	}
	return nil
}

func (api *tripsAPI) createStop(w http.ResponseWriter, r *http.Request) {
	trip, ok := api.editableTrip(w, r)
	if !ok {
		return
	}
	var req stopRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apiError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}
	var params store.StopParams
	if err := req.merge(&params); err != nil {
		apiError(w, http.StatusBadRequest, "invalid", err.Error())
		return
	}
	stop, err := api.trips.CreateStop(r.Context(), trip.ID, params)
	if err != nil {
		apiInternalError(w, "create stop", err)
		return
	}
	writeJSON(w, http.StatusCreated, toStopJSON(stop))
}

func (api *tripsAPI) updateStop(w http.ResponseWriter, r *http.Request) {
	trip, ok := api.editableTrip(w, r)
	if !ok {
		return
	}
	stopID, err := uuid.Parse(r.PathValue("stopID"))
	if err != nil {
		apiError(w, http.StatusNotFound, "not_found", "stop not found")
		return
	}
	current, err := api.trips.StopByID(r.Context(), trip.ID, stopID)
	if errors.Is(err, store.ErrNotFound) {
		apiError(w, http.StatusNotFound, "not_found", "stop not found")
		return
	}
	if err != nil {
		apiInternalError(w, "load stop", err)
		return
	}

	var req stopRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apiError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}
	params := store.StopParams{
		Name: current.Name, Lat: current.Lat, Lon: current.Lon,
		ArrivalDate: current.ArrivalDate, DepartureDate: current.DepartureDate, Notes: current.Notes,
	}
	if err := req.merge(&params); err != nil {
		apiError(w, http.StatusBadRequest, "invalid", err.Error())
		return
	}
	updated, err := api.trips.UpdateStop(r.Context(), trip.ID, stopID, params)
	if err != nil {
		apiInternalError(w, "update stop", err)
		return
	}
	writeJSON(w, http.StatusOK, toStopJSON(updated))
}

func (api *tripsAPI) deleteStop(w http.ResponseWriter, r *http.Request) {
	trip, ok := api.editableTrip(w, r)
	if !ok {
		return
	}
	stopID, err := uuid.Parse(r.PathValue("stopID"))
	if err != nil {
		apiError(w, http.StatusNotFound, "not_found", "stop not found")
		return
	}
	switch err := api.trips.DeleteStop(r.Context(), trip.ID, stopID); {
	case errors.Is(err, store.ErrNotFound):
		apiError(w, http.StatusNotFound, "not_found", "stop not found")
	case err != nil:
		apiInternalError(w, "delete stop", err)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

func (api *tripsAPI) reorderStops(w http.ResponseWriter, r *http.Request) {
	trip, ok := api.editableTrip(w, r)
	if !ok {
		return
	}
	var req struct {
		IDs []uuid.UUID `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apiError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}
	switch err := api.trips.ReorderStops(r.Context(), trip.ID, req.IDs); {
	case errors.Is(err, store.ErrNotFound):
		apiError(w, http.StatusBadRequest, "invalid", "ids must be a permutation of the trip's stops")
	case err != nil:
		apiInternalError(w, "reorder stops", err)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}
