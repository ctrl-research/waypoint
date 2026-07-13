package server

import (
	"math"
	"net/http"
	"sort"
	"strings"

	"github.com/google/uuid"

	"github.com/ctrl-research/waypoint/internal/auth"
)

// handleStats aggregates everything the stats page (#26) needs in one call:
// trip totals, days on the road, planned route distance, trips per year, and
// every located stop (the map highlights countries/cities from these
// client-side, where the country polygons already live).
func (api *tripsAPI) handleStats(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFrom(r.Context())

	trips, err := api.trips.ListAccessible(r.Context(), user.ID)
	if err != nil {
		apiInternalError(w, "list trips", err)
		return
	}
	stops, err := api.trips.ListLocatedStops(r.Context(), user.ID)
	if err != nil {
		apiInternalError(w, "list stops", err)
		return
	}

	byStatus := map[string]int{}
	daysOnRoad := 0
	perYear := map[int]int{}
	for _, t := range trips {
		byStatus[string(t.Trip.Status)]++
		if t.Trip.StartDate != nil {
			perYear[t.Trip.StartDate.Year()]++
			// Guard against inverted ranges from rows that predate date
			// validation.
			if t.Trip.EndDate != nil && !t.Trip.EndDate.Before(*t.Trip.StartDate) {
				daysOnRoad += int(t.Trip.EndDate.Sub(*t.Trip.StartDate).Hours()/24) + 1
			}
		}
	}

	type yearCount struct {
		Year  int `json:"year"`
		Count int `json:"count"`
	}
	years := make([]yearCount, 0, len(perYear))
	for y, c := range perYear {
		years = append(years, yearCount{y, c})
	}
	sort.Slice(years, func(i, j int) bool { return years[i].Year < years[j].Year })

	// Planned distance: haversine legs between consecutive stops per trip.
	distanceKm := 0.0
	var prevTrip uuid.UUID
	var prevLat, prevLon float64
	hasPrev := false
	cities := map[string]struct{}{}
	type stopJSON struct {
		Name      string  `json:"name"`
		Lat       float64 `json:"lat"`
		Lon       float64 `json:"lon"`
		TripTitle string  `json:"tripTitle"`
	}
	stopsOut := make([]stopJSON, 0, len(stops))
	for _, s := range stops {
		lat, lon := *s.Lat, *s.Lon
		if hasPrev && s.TripID == prevTrip {
			distanceKm += haversineKm(prevLat, prevLon, lat, lon)
		}
		prevTrip, prevLat, prevLon, hasPrev = s.TripID, lat, lon, true
		cities[strings.ToLower(strings.TrimSpace(s.Name))] = struct{}{}
		stopsOut = append(stopsOut, stopJSON{Name: s.Name, Lat: lat, Lon: lon, TripTitle: s.TripTitle})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"totals": map[string]any{
			"trips":             len(trips),
			"planning":          byStatus["planning"],
			"active":            byStatus["active"],
			"completed":         byStatus["completed"],
			"daysOnRoad":        daysOnRoad,
			"plannedDistanceKm": math.Round(distanceKm),
			"cities":            len(cities),
		},
		"tripsPerYear": years,
		"stops":        stopsOut,
	})
}

func haversineKm(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadiusKm = 6371
	rad := func(deg float64) float64 { return deg * math.Pi / 180 }
	dLat, dLon := rad(lat2-lat1), rad(lon2-lon1)
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(rad(lat1))*math.Cos(rad(lat2))*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * earthRadiusKm * math.Asin(math.Sqrt(a))
}
