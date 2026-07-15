package server

import (
	"net/http"
	"strings"

	"github.com/ctrl-research/waypoint/internal/geocode"
)

// handleGeocode proxies place search for the stop-picker. Auth-guarded so
// the instance doesn't become an open geocoding relay.
func handleGeocode(geo *geocode.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := strings.TrimSpace(r.URL.Query().Get("q"))
		if len(q) < 2 {
			apiError(w, http.StatusBadRequest, "invalid", "q must be at least 2 characters")
			return
		}
		results, err := geo.Search(r.Context(), q, 5, r.URL.Query().Get("type"))
		if err != nil {
			apiInternalError(w, "geocode", err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"results": results})
	}
}
