package server

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/ctrl-research/waypoint/internal/auth"
	"github.com/ctrl-research/waypoint/internal/photos"
	"github.com/ctrl-research/waypoint/internal/store"
)

// tripsAPI serves owner-scoped CRUD for trips, stops, itinerary items, and
// the journal. Trips are visible to their owner only until sharing lands
// (M6, #23).
type tripsAPI struct {
	trips   *store.Trips
	users   *store.Users
	photos  *photos.Store
	tileURL string
}

func (api *tripsAPI) routes(mux *http.ServeMux) {
	protected := func(h http.HandlerFunc) http.Handler { return auth.RequireUser(h) }

	mux.Handle("GET /api/v1/trips", protected(api.list))
	mux.Handle("POST /api/v1/trips", protected(api.create))
	mux.Handle("GET /api/v1/trips/{tripID}", protected(api.get))
	mux.Handle("PATCH /api/v1/trips/{tripID}", protected(api.update))
	mux.Handle("DELETE /api/v1/trips/{tripID}", protected(api.delete))

	mux.Handle("POST /api/v1/trips/{tripID}/stops", protected(api.createStop))
	mux.Handle("PUT /api/v1/trips/{tripID}/stops/order", protected(api.reorderStops))
	mux.Handle("PATCH /api/v1/trips/{tripID}/stops/{stopID}", protected(api.updateStop))
	mux.Handle("DELETE /api/v1/trips/{tripID}/stops/{stopID}", protected(api.deleteStop))

	mux.Handle("POST /api/v1/trips/{tripID}/items", protected(api.createItem))
	mux.Handle("PUT /api/v1/trips/{tripID}/items/order", protected(api.reorderItems))
	mux.Handle("PATCH /api/v1/trips/{tripID}/items/{itemID}", protected(api.updateItem))
	mux.Handle("DELETE /api/v1/trips/{tripID}/items/{itemID}", protected(api.deleteItem))

	mux.Handle("GET /api/v1/trips/{tripID}/journal", protected(api.listJournal))
	mux.Handle("POST /api/v1/trips/{tripID}/journal", protected(api.createEntry))
	mux.Handle("PATCH /api/v1/trips/{tripID}/journal/{entryID}", protected(api.updateEntry))
	mux.Handle("DELETE /api/v1/trips/{tripID}/journal/{entryID}", protected(api.deleteEntry))
	mux.Handle("POST /api/v1/trips/{tripID}/journal/{entryID}/photos", protected(api.uploadPhoto))
	mux.Handle("DELETE /api/v1/trips/{tripID}/photos/{photoID}", protected(api.deletePhoto))
	mux.Handle("GET /api/v1/photos/{photoID}", protected(api.servePhoto))

	mux.Handle("GET /api/v1/trips/{tripID}/members", protected(api.listMembers))
	mux.Handle("POST /api/v1/trips/{tripID}/members", protected(api.addMember))
	mux.Handle("DELETE /api/v1/trips/{tripID}/members/{userID}", protected(api.removeMember))

	mux.Handle("GET /api/v1/stats", protected(api.handleStats))

	mux.Handle("GET /api/v1/homes", protected(api.listHomes))
	mux.Handle("POST /api/v1/homes", protected(api.createHome))
	mux.Handle("DELETE /api/v1/homes/{homeID}", protected(api.deleteHome))

	mux.Handle("GET /api/v1/trips/{tripID}/export/gpx", protected(api.exportGPX))
	mux.Handle("GET /api/v1/trips/{tripID}/export/geojson", protected(api.exportGeoJSON))
	mux.Handle("GET /api/v1/trips/{tripID}/export/markdown", protected(api.exportMarkdown))

	mux.Handle("GET /api/v1/trips/{tripID}/shares", protected(api.listShares))
	mux.Handle("POST /api/v1/trips/{tripID}/shares", protected(api.createShare))
	mux.Handle("DELETE /api/v1/trips/{tripID}/shares/{shareID}", protected(api.revokeShare))

	// Public, token-scoped read-only access — deliberately NOT session-guarded.
	mux.HandleFunc("GET /api/v1/public/{token}", api.servePublicTrip)
	mux.HandleFunc("GET /api/v1/public/{token}/photos/{photoID}", api.servePublicPhoto)
}

// ---- JSON shapes ----------------------------------------------------------

const dateFormat = "2006-01-02"

type tripJSON struct {
	ID          uuid.UUID `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	StartDate   *string   `json:"startDate"`
	EndDate     *string   `json:"endDate"`
	CoverPhoto  *string   `json:"coverPhoto"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	// Role is the requesting user's role on this trip: owner|editor|viewer.
	Role string `json:"role"`
}

func toTripJSON(t store.Trip, role string) tripJSON {
	return tripJSON{
		ID: t.ID, Title: t.Title, Description: t.Description, Status: string(t.Status),
		StartDate: formatDate(t.StartDate), EndDate: formatDate(t.EndDate),
		CoverPhoto: t.CoverPhoto, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt,
		Role: role,
	}
}

type stopJSON struct {
	ID            uuid.UUID `json:"id"`
	Name          string    `json:"name"`
	Lat           *float64  `json:"lat"`
	Lon           *float64  `json:"lon"`
	ArrivalDate   *string   `json:"arrivalDate"`
	DepartureDate *string   `json:"departureDate"`
	Position      int32     `json:"position"`
	Notes         string    `json:"notes"`
}

func toStopJSON(s store.Stop) stopJSON {
	return stopJSON{
		ID: s.ID, Name: s.Name, Lat: s.Lat, Lon: s.Lon,
		ArrivalDate: formatDate(s.ArrivalDate), DepartureDate: formatDate(s.DepartureDate),
		Position: s.Position, Notes: s.Notes,
	}
}

type itemJSON struct {
	ID                uuid.UUID  `json:"id"`
	StopID            *uuid.UUID `json:"stopId"`
	DestinationStopID *uuid.UUID `json:"destinationStopId"`
	OriginHomeID      *uuid.UUID `json:"originHomeId"`
	DestinationHomeID *uuid.UUID `json:"destinationHomeId"`
	Day               string     `json:"day"`
	StartTime         *string    `json:"startTime"`
	EndTime           *string    `json:"endTime"`
	Title             string     `json:"title"`
	Category          string     `json:"category"`
	Notes             string     `json:"notes"`
	CostCents         *int64     `json:"costCents"`
	Currency          *string    `json:"currency"`
	Position          int32      `json:"position"`
}

func toItemJSON(it store.ItineraryItem) itemJSON {
	return itemJSON{
		ID: it.ID, StopID: it.StopID, DestinationStopID: it.DestinationStopID,
		OriginHomeID: it.OriginHomeID, DestinationHomeID: it.DestinationHomeID,
		Day: it.Day.Format(dateFormat), StartTime: nilIfEmpty(it.StartTime), EndTime: nilIfEmpty(it.EndTime),
		Title: it.Title, Category: string(it.Category), Notes: it.Notes,
		CostCents: it.CostCents, Currency: it.Currency, Position: it.Position,
	}
}

func formatDate(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format(dateFormat)
	return &s
}

// ---- trips ----------------------------------------------------------------

// tripRequest is used for both POST (create) and PATCH (partial update):
// nil pointers mean "keep the current value". Dates use "YYYY-MM-DD"; an
// empty string clears the field.
type tripRequest struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Status      *string `json:"status"`
	StartDate   *string `json:"startDate"`
	EndDate     *string `json:"endDate"`
	CoverPhoto  *string `json:"coverPhoto"`
}

// merge applies the request on top of p, validating as it goes.
func (req tripRequest) merge(p *store.TripParams) error {
	if req.Title != nil {
		p.Title = *req.Title
	}
	if p.Title == "" {
		return errors.New("title is required")
	}
	if req.Description != nil {
		p.Description = *req.Description
	}
	if req.Status != nil {
		if !store.ValidTripStatus(*req.Status) {
			return errors.New("status must be planning, active, or completed")
		}
		p.Status = store.TripStatus(*req.Status)
	}
	var err error
	if p.StartDate, err = mergeDate(req.StartDate, p.StartDate, "startDate"); err != nil {
		return err
	}
	if p.EndDate, err = mergeDate(req.EndDate, p.EndDate, "endDate"); err != nil {
		return err
	}
	if p.StartDate != nil && p.EndDate != nil && p.EndDate.Before(*p.StartDate) {
		return errors.New("endDate must be on or after startDate")
	}
	if req.CoverPhoto != nil {
		p.CoverPhoto = nilIfEmpty(*req.CoverPhoto)
	}
	return nil
}

func (api *tripsAPI) list(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFrom(r.Context())
	trips, err := api.trips.ListAccessible(r.Context(), user.ID)
	if err != nil {
		apiInternalError(w, "list trips", err)
		return
	}
	out := make([]tripJSON, 0, len(trips))
	for _, t := range trips {
		out = append(out, toTripJSON(t.Trip, t.Role))
	}
	writeJSON(w, http.StatusOK, map[string]any{"trips": out})
}

func (api *tripsAPI) create(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFrom(r.Context())
	var req tripRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apiError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}
	params := store.TripParams{Status: store.TripPlanning}
	if err := req.merge(&params); err != nil {
		apiError(w, http.StatusBadRequest, "invalid", err.Error())
		return
	}
	trip, err := api.trips.Create(r.Context(), user.ID, params)
	if err != nil {
		apiInternalError(w, "create trip", err)
		return
	}
	writeJSON(w, http.StatusCreated, toTripJSON(trip, "owner"))
}

func (api *tripsAPI) get(w http.ResponseWriter, r *http.Request) {
	trip, role, ok := api.tripAccess(w, r, "viewer")
	if !ok {
		return
	}
	stops, err := api.trips.ListStops(r.Context(), trip.ID)
	if err != nil {
		apiInternalError(w, "list stops", err)
		return
	}
	items, err := api.trips.ListItems(r.Context(), trip.ID)
	if err != nil {
		apiInternalError(w, "list items", err)
		return
	}
	stopsOut := make([]stopJSON, 0, len(stops))
	for _, s := range stops {
		stopsOut = append(stopsOut, toStopJSON(s))
	}
	itemsOut := make([]itemJSON, 0, len(items))
	for _, it := range items {
		itemsOut = append(itemsOut, toItemJSON(it))
	}
	tripHomes, err := api.trips.ListTripHomes(r.Context(), trip.ID)
	if err != nil {
		apiInternalError(w, "list trip homes", err)
		return
	}
	homesOut := make([]map[string]any, 0, len(tripHomes))
	for _, th := range tripHomes {
		homesOut = append(homesOut, map[string]any{"id": th.ID, "name": th.Name})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"trip": toTripJSON(trip, role), "stops": stopsOut, "items": itemsOut, "homes": homesOut,
	})
}

func (api *tripsAPI) update(w http.ResponseWriter, r *http.Request) {
	trip, role, ok := api.tripAccess(w, r, "editor")
	if !ok {
		return
	}
	var req tripRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apiError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}
	params := store.TripParams{
		Title: trip.Title, Description: trip.Description, Status: trip.Status,
		StartDate: trip.StartDate, EndDate: trip.EndDate, CoverPhoto: trip.CoverPhoto,
	}
	if err := req.merge(&params); err != nil {
		apiError(w, http.StatusBadRequest, "invalid", err.Error())
		return
	}
	updated, err := api.trips.Update(r.Context(), trip.ID, params)
	if err != nil {
		apiInternalError(w, "update trip", err)
		return
	}
	writeJSON(w, http.StatusOK, toTripJSON(updated, role))
}

func (api *tripsAPI) delete(w http.ResponseWriter, r *http.Request) {
	trip, _, ok := api.tripAccess(w, r, "owner")
	if !ok {
		return
	}
	if err := api.trips.Delete(r.Context(), trip.ID); err != nil {
		apiInternalError(w, "delete trip", err)
		return
	}
	// The DB cascade removed the photo rows; remove the files too.
	_ = api.photos.RemoveTrip(trip.ID)
	w.WriteHeader(http.StatusNoContent)
}

// ---- helpers ---------------------------------------------------------------

func roleRank(role string) int {
	switch role {
	case "owner":
		return 3
	case "editor":
		return 2
	case "viewer":
		return 1
	}
	return 0
}

// tripAccess resolves {tripID} and enforces the minimum role. Missing trips
// and no-access are both 404 so trip IDs don't leak; an insufficient role on
// a trip the user CAN see is 403.
func (api *tripsAPI) tripAccess(w http.ResponseWriter, r *http.Request, min string) (store.Trip, string, bool) {
	user, _ := auth.UserFrom(r.Context())
	id, err := uuid.Parse(r.PathValue("tripID"))
	if err != nil {
		apiError(w, http.StatusNotFound, "not_found", "trip not found")
		return store.Trip{}, "", false
	}
	trip, role, err := api.trips.WithRole(r.Context(), id, user.ID)
	if errors.Is(err, store.ErrNotFound) || (err == nil && role == "") {
		apiError(w, http.StatusNotFound, "not_found", "trip not found")
		return store.Trip{}, "", false
	}
	if err != nil {
		apiInternalError(w, "load trip", err)
		return store.Trip{}, "", false
	}
	if roleRank(role) < roleRank(min) {
		apiError(w, http.StatusForbidden, "forbidden", "your role on this trip does not allow that")
		return store.Trip{}, "", false
	}
	return trip, role, true
}

// editableTrip is the common case: any mutation needs at least editor.
func (api *tripsAPI) editableTrip(w http.ResponseWriter, r *http.Request) (store.Trip, bool) {
	trip, _, ok := api.tripAccess(w, r, "editor")
	return trip, ok
}

func mergeDate(req *string, current *time.Time, field string) (*time.Time, error) {
	if req == nil {
		return current, nil
	}
	if *req == "" {
		return nil, nil
	}
	t, err := time.Parse(dateFormat, *req)
	if err != nil {
		return nil, errors.New(field + " must be YYYY-MM-DD")
	}
	return &t, nil
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func apiError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]string{"code": code, "message": message},
	})
}

func apiInternalError(w http.ResponseWriter, what string, err error) {
	slog.Error(what, "err", err)
	apiError(w, http.StatusInternalServerError, "internal", "something went wrong")
}
