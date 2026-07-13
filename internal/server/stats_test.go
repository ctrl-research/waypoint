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
	call(t, h, alice, "POST", p1+"/stops", `{"name":"Paris","lat":48.8566,"lon":2.3522}`)
	call(t, h, alice, "POST", p1+"/stops", `{"name":"Lyon","lat":45.7640,"lon":4.8357}`)

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
