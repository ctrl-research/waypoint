package server

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ctrl-research/waypoint/internal/auth"
	"github.com/ctrl-research/waypoint/internal/geocode"
	"github.com/ctrl-research/waypoint/internal/store"
	"github.com/ctrl-research/waypoint/internal/store/storetest"
)

func TestCalendarICS(t *testing.T) {
	h, alice, bob := setup(t)

	_, trip := call(t, h, alice, "POST", "/api/v1/trips",
		`{"title":"Alps; summer","startDate":"2027-07-10","endDate":"2027-07-14"}`)
	tripPath := "/api/v1/trips/" + trip["id"].(string)
	// A timed item with an address that needs RFC 5545 escaping.
	if code, resp := call(t, h, alice, "POST", tripPath+"/items",
		`{"title":"Fondue","day":"2027-07-11","startTime":"19:00","endTime":"21:00","address":"Rue du Lac 3, Geneva"}`); code != 201 {
		t.Fatalf("create item: code = %d %v", code, resp)
	}

	rawGET := func(path string, cookie *http.Cookie) (int, string) {
		req := httptest.NewRequest("GET", path, nil)
		if cookie != nil {
			req.AddCookie(cookie)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code, rec.Body.String()
	}

	t.Run("per-trip export", func(t *testing.T) {
		code, body := rawGET(tripPath+"/export/ics", alice)
		if code != 200 {
			t.Fatalf("export ics: code = %d", code)
		}
		for _, want := range []string{
			"BEGIN:VCALENDAR",
			"SUMMARY:✈ Alps\\; summer",
			"DTSTART;VALUE=DATE:20270710",
			"DTEND;VALUE=DATE:20270715", // exclusive end
			"SUMMARY:Fondue",
			"DTSTART:20270711T190000",
			"DTEND:20270711T210000",
			"LOCATION:Rue du Lac 3\\, Geneva",
		} {
			if !strings.Contains(body, want) {
				t.Fatalf("ics missing %q in:\n%s", want, body)
			}
		}
	})

	t.Run("feed token lifecycle", func(t *testing.T) {
		_, cur := call(t, h, alice, "GET", "/api/v1/calendar/token", "")
		if cur["token"] != nil {
			t.Fatalf("initial token = %v, want null", cur["token"])
		}
		code, created := call(t, h, alice, "POST", "/api/v1/calendar/token", "")
		if code != 201 || created["token"] == "" {
			t.Fatalf("create token: code = %d %v", code, created)
		}
		token := created["token"].(string)

		// The feed needs no session and contains the trip and its item.
		code, body := rawGET("/api/v1/calendar/"+token+"/waypoint.ics", nil)
		if code != 200 || !strings.Contains(body, "SUMMARY:✈ Alps\\; summer") || !strings.Contains(body, "SUMMARY:Fondue") {
			t.Fatalf("feed: code = %d body:\n%s", code, body)
		}

		if code, _ := rawGET("/api/v1/calendar/nope/waypoint.ics", nil); code != 404 {
			t.Fatalf("bad token: code = %d, want 404", code)
		}

		// Regenerating rotates: old token dies.
		_, again := call(t, h, alice, "POST", "/api/v1/calendar/token", "")
		if again["token"] == token {
			t.Fatalf("regenerated token unchanged")
		}
		if code, _ := rawGET("/api/v1/calendar/"+token+"/waypoint.ics", nil); code != 404 {
			t.Fatalf("old token after rotate: code = %d, want 404", code)
		}

		// Disabling kills the feed entirely.
		if code, _ := call(t, h, alice, "DELETE", "/api/v1/calendar/token", ""); code != 204 {
			t.Fatalf("delete token: code = %d", code)
		}
		code, cleared := call(t, h, alice, "GET", "/api/v1/calendar/token", "")
		if code != 200 || cleared["token"] != nil {
			t.Fatalf("token after delete = %v", cleared["token"])
		}
	})

	t.Run("feed is scoped to its user", func(t *testing.T) {
		_, created := call(t, h, bob, "POST", "/api/v1/calendar/token", "")
		code, body := rawGET(fmt.Sprintf("/api/v1/calendar/%s/waypoint.ics", created["token"]), nil)
		if code != 200 || strings.Contains(body, "Alps") {
			t.Fatalf("bob's feed leaked alice's trip: code = %d body:\n%s", code, body)
		}
	})
}

func TestCalendarICSTimezone(t *testing.T) {
	pool := storetest.Pool(t)
	ctx := context.Background()

	users := store.NewUsers(pool)
	sessions := store.NewSessions(pool)
	authSvc, err := auth.NewService(ctx, users, sessions, auth.Options{BaseURL: "http://localhost:8080"})
	if err != nil {
		t.Fatalf("auth.NewService: %v", err)
	}

	u, err := users.Create(ctx, store.CreateUserParams{Email: "tz@example.com", GoogleSub: ptrStr("tz@example.com")})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	h := sha256.Sum256([]byte("tok-tz"))
	if err := sessions.Create(ctx, h[:], u.ID, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("create session: %v", err)
	}
	cookie := &http.Cookie{Name: "waypoint_session", Value: "tok-tz"}

	handler := New(pool, authSvc, geocode.New("http://unused.invalid", ""),
		Options{TileURL: "https://tiles.test/{z}/{x}/{y}.png", DataDir: t.TempDir(), EnableMCP: true, Timezone: "America/Edmonton"})

	// Create trip and item with 8am start time
	_, trip := call(t, handler, cookie, "POST", "/api/v1/trips",
		`{"title":"Vancouver trip","startDate":"2027-07-22","endDate":"2027-07-25"}`)
	tripPath := "/api/v1/trips/" + trip["id"].(string)
	if code, resp := call(t, handler, cookie, "POST", tripPath+"/items",
		`{"title":"Morning meeting","day":"2027-07-22","startTime":"08:00","endTime":"09:30"}`); code != 201 {
		t.Fatalf("create item: code = %d %v", code, resp)
	}

	req := httptest.NewRequest("GET", tripPath+"/export/ics", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("export ics: code = %d", rec.Code)
	}
	body := rec.Body.String()

	// 8am MDT (UTC-6) = 14:00 UTC, 9:30am MDT = 15:30 UTC
	for _, want := range []string{
		"DTSTART:20270722T140000Z",
		"DTEND:20270722T153000Z",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("ics missing %q in:\n%s", want, body)
		}
	}

	// Verify all-day trip span is still floating (no Z)
	if !strings.Contains(body, "DTSTART;VALUE=DATE:20270722") {
		t.Fatalf("all-day event should not have timezone in:\n%s", body)
	}
}

func ptrStr(s string) *string { return &s }
