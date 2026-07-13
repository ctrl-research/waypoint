package server

import (
	"fmt"
	"testing"
)

func TestStats(t *testing.T) {
	h, alice, bob := setup(t)

	// Trip 1: dated, two located stops (Paris → Lyon ≈ 392km), one repeated city name.
	_, t1 := call(t, h, alice, "POST", "/api/v1/trips", `{"title":"France","startDate":"2026-05-01","endDate":"2026-05-08","status":"completed"}`)
	p1 := "/api/v1/trips/" + t1["id"].(string)
	_, paris := call(t, h, alice, "POST", p1+"/stops", `{"name":"Paris","lat":48.8566,"lon":2.3522}`)
	_, lyon := call(t, h, alice, "POST", p1+"/stops", `{"name":"Lyon","lat":45.7640,"lon":4.8357}`)
	// Home in Toronto: home → Paris is ≈6000km.
	code0, home := call(t, h, alice, "POST", "/api/v1/homes", `{"name":"Toronto","lat":43.65,"lon":-79.38}`)
	if code0 != 201 {
		t.Fatalf("create home: code = %d %v", code0, home)
	}
	// Flight home → Paris, then a train Paris → Lyon (≈392km), 10:00–11:15.
	if code, resp := call(t, h, alice, "POST", p1+"/items", fmt.Sprintf(
		`{"title":"AC880","day":"2026-05-01","category":"flight","originHomeId":%q,"destinationStopId":%q}`,
		home["id"], paris["id"])); code != 201 {
		t.Fatalf("create home flight: code = %d %v", code, resp)
	}
	if code, resp := call(t, h, alice, "POST", p1+"/items", fmt.Sprintf(
		`{"title":"TGV","day":"2026-05-02","category":"train","startTime":"10:00","endTime":"11:15","stopId":%q,"destinationStopId":%q}`,
		paris["id"], lyon["id"])); code != 201 {
		t.Fatalf("create train: code = %d %v", code, resp)
	}

	// Trip 2: dated next year, one stop named Paris again (dedupes as a city).
	_, t2 := call(t, h, alice, "POST", "/api/v1/trips", `{"title":"Encore","startDate":"2027-04-01","endDate":"2027-04-03"}`)
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
	if totals["daysOnRoad"].(float64) != 11 { // 8 + 3
		t.Fatalf("daysOnRoad = %v, want 11", totals["daysOnRoad"])
	}
	if totals["cities"].(float64) != 2 { // paris, lyon (dedup + unlocated skipped)
		t.Fatalf("cities = %v, want 2", totals["cities"])
	}
	km := totals["plannedDistanceKm"].(float64)
	if km < 380 || km > 405 {
		t.Fatalf("plannedDistanceKm = %v, want ≈392", km)
	}
	if n := len(stats["stops"].([]any)); n != 3 {
		t.Fatalf("stops = %d, want 3 located", n)
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
	if len(years) != 2 || years[0].(map[string]any)["year"].(float64) != 2026 {
		t.Fatalf("tripsPerYear = %v", years)
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
