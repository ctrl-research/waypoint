package server

import (
	"fmt"
	"testing"
	"time"
)

func TestStats(t *testing.T) {
	h, alice, bob := setup(t)

	// Dates pivot on today so the travelled/planned split stays stable:
	// trip 1 well in the past, trip 2 well in the future (±400 days also
	// guarantees they land in different years).
	day := func(offset int) string { return time.Now().UTC().AddDate(0, 0, offset).Format("2006-01-02") }

	// Trip 1 (travelled): Paris → Lyon covered by a train, then Lyon → Nice
	// overland (≈298km); 8 days.
	_, t1 := call(t, h, alice, "POST", "/api/v1/trips", fmt.Sprintf(
		`{"title":"France","startDate":%q,"endDate":%q,"status":"completed"}`, day(-400), day(-393)))
	p1 := "/api/v1/trips/" + t1["id"].(string)
	_, paris := call(t, h, alice, "POST", p1+"/stops", `{"name":"Paris","lat":48.8566,"lon":2.3522}`)
	_, lyon := call(t, h, alice, "POST", p1+"/stops", `{"name":"Lyon","lat":45.7640,"lon":4.8357}`)
	call(t, h, alice, "POST", p1+"/stops", `{"name":"Nice","lat":43.7102,"lon":7.2620}`)
	// Home in Toronto: home → Paris is ≈6000km.
	code0, home := call(t, h, alice, "POST", "/api/v1/homes", `{"name":"Toronto","lat":43.65,"lon":-79.38}`)
	if code0 != 201 {
		t.Fatalf("create home: code = %d %v", code0, home)
	}
	// Flight home → Paris, then a train Paris → Lyon (≈392km), 10:00–11:15.
	if code, resp := call(t, h, alice, "POST", p1+"/items", fmt.Sprintf(
		`{"title":"AC880","day":%q,"category":"flight","originHomeId":%q,"destinationStopId":%q}`,
		day(-400), home["id"], paris["id"])); code != 201 {
		t.Fatalf("create home flight: code = %d %v", code, resp)
	}
	if code, resp := call(t, h, alice, "POST", p1+"/items", fmt.Sprintf(
		`{"title":"TGV","day":%q,"category":"train","startTime":"10:00","endTime":"11:15","stopId":%q,"destinationStopId":%q}`,
		day(-399), paris["id"], lyon["id"])); code != 201 {
		t.Fatalf("create train: code = %d %v", code, resp)
	}

	// Trip 2 (planned): one stop named Paris again — already travelled, so
	// it must not count as a planned city; 3 days.
	_, t2 := call(t, h, alice, "POST", "/api/v1/trips", fmt.Sprintf(
		`{"title":"Encore","startDate":%q,"endDate":%q}`, day(400), day(402)))
	p2 := "/api/v1/trips/" + t2["id"].(string)
	call(t, h, alice, "POST", p2+"/stops", `{"name":"Paris","lat":48.8566,"lon":2.3522}`)
	// And an unlocated stop that must not count as a city or distance.
	call(t, h, alice, "POST", p2+"/stops", `{"name":"Mystery"}`)

	code, stats := call(t, h, alice, "GET", "/api/v1/stats", "")
	if code != 200 {
		t.Fatalf("stats: code = %d", code)
	}
	totals := stats["totals"].(map[string]any)
	if totals["trips"].(float64) != 2 || totals["completed"].(float64) != 1 {
		t.Fatalf("totals = %v", totals)
	}
	if totals["daysOnRoad"].(float64) != 8 || totals["daysOnRoadPlanned"].(float64) != 3 {
		t.Fatalf("days = %v/%v, want 8 travelled / 3 planned", totals["daysOnRoad"], totals["daysOnRoadPlanned"])
	}
	// paris, lyon, nice travelled; trip 2's paris dedupes against them.
	if totals["cities"].(float64) != 3 || totals["citiesPlanned"].(float64) != 0 {
		t.Fatalf("cities = %v/%v, want 3 travelled / 0 planned", totals["cities"], totals["citiesPlanned"])
	}
	// Paris→Lyon is covered by the train, so only Lyon→Nice counts.
	km := totals["traveledDistanceKm"].(float64)
	if km < 285 || km > 315 {
		t.Fatalf("traveledDistanceKm = %v, want ≈298 (train segment excluded)", km)
	}
	if totals["plannedDistanceKm"].(float64) != 0 {
		t.Fatalf("plannedDistanceKm = %v, want 0 (single planned stop)", totals["plannedDistanceKm"])
	}
	if n := len(stats["stops"].([]any)); n != 4 {
		t.Fatalf("stops = %d, want 4 located", n)
	}
	travelledStops := 0
	for _, raw := range stats["stops"].([]any) {
		if raw.(map[string]any)["travelled"] == true {
			travelledStops++
		}
	}
	if travelledStops != 3 {
		t.Fatalf("travelled stops = %d, want 3", travelledStops)
	}
	flights := stats["flights"].(map[string]any)
	if flights["count"].(float64) != 1 {
		t.Fatalf("flights = %v, want count 1", flights)
	}
	if km := flights["distanceKm"].(float64); km < 5900 || km > 6150 { // Toronto → Paris
		t.Fatalf("flight distanceKm = %v, want ≈6000 (home leg)", km)
	}
	trains := stats["trains"].(map[string]any)
	if trains["count"].(float64) != 1 {
		t.Fatalf("trains = %v, want count 1", trains)
	}
	if km := trains["distanceKm"].(float64); km < 380 || km > 405 {
		t.Fatalf("train distanceKm = %v, want ≈392", km)
	}
	if m := trains["minutes"].(float64); m != 75 {
		t.Fatalf("train minutes = %v, want 75", m)
	}
	years := stats["tripsPerYear"].([]any)
	if len(years) != 2 {
		t.Fatalf("tripsPerYear = %v, want 2 years", years)
	}
	past := years[0].(map[string]any)
	future := years[1].(map[string]any)
	if past["travelled"].(float64) != 1 || past["planned"].(float64) != 0 {
		t.Fatalf("past year split = %v", past)
	}
	if future["travelled"].(float64) != 0 || future["planned"].(float64) != 1 {
		t.Fatalf("future year split = %v", future)
	}

	t.Run("scoped to the requesting user", func(t *testing.T) {
		_, bobStats := call(t, h, bob, "GET", "/api/v1/stats", "")
		if n := bobStats["totals"].(map[string]any)["trips"].(float64); n != 0 {
			t.Fatalf("bob trips = %v, want 0", n)
		}
		// Sharing brings the trip (and its stops) into bob's stats.
		call(t, h, alice, "POST", p1+"/members", fmt.Sprintf(`{"email":%q,"role":"viewer"}`, "bob@example.com"))
		_, bobStats = call(t, h, bob, "GET", "/api/v1/stats", "")
		if n := bobStats["totals"].(map[string]any)["trips"].(float64); n != 1 {
			t.Fatalf("bob trips after share = %v, want 1", n)
		}
	})
}
