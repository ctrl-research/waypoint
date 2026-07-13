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

type homeJSON struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Lat       float64   `json:"lat"`
	Lon       float64   `json:"lon"`
	CreatedAt time.Time `json:"createdAt"`
}

func toHomeJSON(h store.Home) homeJSON {
	return homeJSON{ID: h.ID, Name: h.Name, Lat: h.Lat, Lon: h.Lon, CreatedAt: h.CreatedAt}
}

func (api *tripsAPI) listHomes(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFrom(r.Context())
	homes, err := api.trips.ListHomes(r.Context(), user.ID)
	if err != nil {
		apiInternalError(w, "list homes", err)
		return
	}
	out := make([]homeJSON, 0, len(homes))
	for _, h := range homes {
		out = append(out, toHomeJSON(h))
	}
	writeJSON(w, http.StatusOK, map[string]any{"homes": out})
}

func (api *tripsAPI) createHome(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFrom(r.Context())
	var req struct {
		Name string  `json:"name"`
		Lat  float64 `json:"lat"`
		Lon  float64 `json:"lon"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		apiError(w, http.StatusBadRequest, "bad_request", "name, lat, and lon are required")
		return
	}
	if req.Lat < -90 || req.Lat > 90 || req.Lon < -180 || req.Lon > 180 {
		apiError(w, http.StatusBadRequest, "invalid", "lat/lon out of range")
		return
	}
	home, err := api.trips.CreateHome(r.Context(), user.ID, req.Name, req.Lat, req.Lon)
	if err != nil {
		apiInternalError(w, "create home", err)
		return
	}
	writeJSON(w, http.StatusCreated, toHomeJSON(home))
}

func (api *tripsAPI) deleteHome(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFrom(r.Context())
	homeID, err := uuid.Parse(r.PathValue("homeID"))
	if err != nil {
		apiError(w, http.StatusNotFound, "not_found", "home not found")
		return
	}
	switch err := api.trips.DeleteHome(r.Context(), user.ID, homeID); {
	case errors.Is(err, store.ErrNotFound):
		apiError(w, http.StatusNotFound, "not_found", "home not found")
	case err != nil:
		apiInternalError(w, "delete home", err)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}
