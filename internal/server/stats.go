package server

import (
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/ctrl-research/waypoint/internal/auth"
	"github.com/ctrl-research/waypoint/internal/store"
)

// handleStats aggregates everything the stats page (#26) needs in one call:
// trip totals, days on the road, route distance, trips per year, and every
// located stop (the map highlights countries/cities from these client-side,
// where the country polygons already live). Trip-derived numbers split into
// travelled (the trip has started) and planned (#53).
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

	// A trip counts as travelled once it has started.
	today := time.Now().UTC().Truncate(24 * time.Hour)
	travelledTrip := func(start *time.Time) bool {
		return start != nil && !start.After(today)
	}

	byStatus := map[string]int{}
	daysTravelled, daysPlanned := 0, 0
	type yearSplit struct {
		Year      int `json:"year"`
		Travelled int `json:"travelled"`
		Planned   int `json:"planned"`
	}
	perYear := map[int]*yearSplit{}
	for _, t := range trips {
		byStatus[string(t.Trip.Status)]++
		if t.Trip.StartDate == nil {
			continue
		}
		y := perYear[t.Trip.StartDate.Year()]
		if y == nil {
			y = &yearSplit{Year: t.Trip.StartDate.Year()}
			perYear[y.Year] = y
		}
		days := 0
		// Guard against inverted ranges from rows that predate date
		// validation.
		if t.Trip.EndDate != nil && !t.Trip.EndDate.Before(*t.Trip.StartDate) {
			days = int(t.Trip.EndDate.Sub(*t.Trip.StartDate).Hours()/24) + 1
		}
		if travelledTrip(t.Trip.StartDate) {
			y.Travelled++
			daysTravelled += days
		} else {
			y.Planned++
			daysPlanned += days
		}
	}

	years := make([]yearSplit, 0, len(perYear))
	for _, y := range perYear {
		years = append(years, *y)
	}
	sort.Slice(years, func(i, j int) bool { return years[i].Year < years[j].Year })

	// Flight/train legs cover stop pairs; those segments leave the overland
	// route distance (#64). Keyed by trip + unordered stop pair.
	covered := map[string]struct{}{}
	pairKey := func(tripID uuid.UUID, a, b uuid.UUID) string {
		if b.String() < a.String() {
			a, b = b, a
		}
		return tripID.String() + "/" + a.String() + "/" + b.String()
	}
	for _, leg := range travelLegs {
		if leg.StopID != nil && leg.DestinationStopID != nil {
			covered[pairKey(leg.TripID, *leg.StopID, *leg.DestinationStopID)] = struct{}{}
		}
	}

	// Route distance: haversine between consecutive stops per trip, minus
	// flight/train-covered segments, split travelled vs planned.
	travelledKm, plannedKm := 0.0, 0.0
	var prevTrip uuid.UUID
	var prevStop uuid.UUID
	var prevLat, prevLon float64
	hasPrev := false
	citiesTravelled := map[string]struct{}{}
	citiesPlanned := map[string]struct{}{}
	type stopJSON struct {
		Name      string  `json:"name"`
		Lat       float64 `json:"lat"`
		Lon       float64 `json:"lon"`
		TripTitle string  `json:"tripTitle"`
		Travelled bool    `json:"travelled"`
	}
	stopsOut := make([]stopJSON, 0, len(stops))
	for _, s := range stops {
		lat, lon := *s.Lat, *s.Lon
		travelled := travelledTrip(s.StartDate)
		if hasPrev && s.TripID == prevTrip {
			if _, skip := covered[pairKey(s.TripID, prevStop, s.ID)]; !skip {
				if travelled {
					travelledKm += haversineKm(prevLat, prevLon, lat, lon)
				} else {
					plannedKm += haversineKm(prevLat, prevLon, lat, lon)
				}
			}
		}
		prevTrip, prevStop, prevLat, prevLon, hasPrev = s.TripID, s.ID, lat, lon, true
		city := strings.ToLower(strings.TrimSpace(s.Name))
		if travelled {
			citiesTravelled[city] = struct{}{}
		} else {
			citiesPlanned[city] = struct{}{}
		}
		stopsOut = append(stopsOut, stopJSON{Name: s.Name, Lat: lat, Lon: lon, TripTitle: s.TripTitle, Travelled: travelled})
	}
	// A city already travelled does not also count as planned.
	for city := range citiesTravelled {
		delete(citiesPlanned, city)
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
			"trips":              len(trips),
			"planning":           byStatus["planning"],
			"active":             byStatus["active"],
			"completed":          byStatus["completed"],
			"daysOnRoad":         daysTravelled,
			"daysOnRoadPlanned":  daysPlanned,
			"traveledDistanceKm": math.Round(travelledKm),
			"plannedDistanceKm":  math.Round(plannedKm),
			"cities":             len(citiesTravelled),
			"citiesPlanned":      len(citiesPlanned),
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
