package server

import (
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"

	"github.com/google/uuid"

	"github.com/ctrl-research/waypoint/internal/auth"
	"github.com/ctrl-research/waypoint/internal/store"
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
	travelLegs, err := api.trips.ListTravelLegs(r.Context(), user.ID)
	if err != nil {
		apiInternalError(w, "list travel legs", err)
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

	// Flight/train aggregates: great-circle distance between endpoints
	// (home resolves to the trip owner's home) and naive local-time
	// durations (overnight legs wrap by +24h; timezone shifts make this an
	// approximation).
	type legAgg struct {
		Count   int     `json:"count"`
		Km      float64 `json:"distanceKm"`
		Minutes int     `json:"minutes"`
	}
	aggs := map[store.ItineraryCategory]*legAgg{
		store.CategoryFlight: {}, store.CategoryTrain: {},
	}
	for _, leg := range travelLegs {
		agg, ok := aggs[leg.Category]
		if !ok {
			continue
		}
		agg.Count++
		fromLat, fromLon := coalesce(leg.FromStopLat, leg.FromHomeLat), coalesce(leg.FromStopLon, leg.FromHomeLon)
		toLat, toLon := coalesce(leg.ToStopLat, leg.ToHomeLat), coalesce(leg.ToStopLon, leg.ToHomeLon)
		if fromLat != nil && fromLon != nil && toLat != nil && toLon != nil {
			agg.Km += haversineKm(*fromLat, *fromLon, *toLat, *toLon)
		}
		if m := legMinutes(leg.StartTime, leg.EndTime); m > 0 {
			agg.Minutes += m
		}
	}
	for _, a := range aggs {
		a.Km = math.Round(a.Km)
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
		"flights":      aggs[store.CategoryFlight],
		"trains":       aggs[store.CategoryTrain],
		"tripsPerYear": years,
		"stops":        stopsOut,
	})
}

func coalesce(a, b *float64) *float64 {
	if a != nil {
		return a
	}
	return b
}

// legMinutes computes end-start from "HH:MM" strings; overnight wraps +24h.
func legMinutes(start, end string) int {
	parse := func(s string) (int, bool) {
		var h, m int
		if _, err := fmt.Sscanf(s, "%d:%d", &h, &m); err != nil {
			return 0, false
		}
		return h*60 + m, true
	}
	a, okA := parse(start)
	b, okB := parse(end)
	if !okA || !okB {
		return 0
	}
	d := b - a
	if d < 0 {
		d += 24 * 60
	}
	return d
}

func haversineKm(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadiusKm = 6371
	rad := func(deg float64) float64 { return deg * math.Pi / 180 }
	dLat, dLon := rad(lat2-lat1), rad(lon2-lon1)
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(rad(lat1))*math.Cos(rad(lat2))*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * earthRadiusKm * math.Asin(math.Sqrt(a))
}
