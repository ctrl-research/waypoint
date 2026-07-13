package server

import (
	"context"
	"crypto/sha256"
	"encoding/json"
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

// setup builds the full HTTP handler against a real database and returns
// session cookies for two users, so tests exercise routing, auth middleware,
// and handlers exactly as production does.
func setup(t *testing.T) (http.Handler, *http.Cookie, *http.Cookie) {
	t.Helper()
	pool := storetest.Pool(t)
	ctx := context.Background()

	users := store.NewUsers(pool)
	sessions := store.NewSessions(pool)
	authSvc, err := auth.NewService(ctx, users, sessions, auth.Options{BaseURL: "http://localhost:8080"})
	if err != nil {
		t.Fatalf("auth.NewService: %v", err)
	}

	cookieFor := func(email, token string) *http.Cookie {
		u, err := users.Create(ctx, store.CreateUserParams{Email: email, GoogleSub: &email})
		if err != nil {
			t.Fatalf("create user: %v", err)
		}
		h := sha256.Sum256([]byte(token))
		if err := sessions.Create(ctx, h[:], u.ID, time.Now().Add(time.Hour)); err != nil {
			t.Fatalf("create session: %v", err)
		}
		return &http.Cookie{Name: "waypoint_session", Value: token}
	}

	return New(pool, authSvc, geocode.New("http://unused.invalid", ""),
			Options{TileURL: "https://tiles.test/{z}/{x}/{y}.png", MapStyleURL: "https://style.test/style.json", Language: "en", DataDir: t.TempDir()}),
		cookieFor("alice@example.com", "tok-alice"), cookieFor("bob@example.com", "tok-bob")
}

// call sends a JSON request and decodes the JSON response (nil for 204s).
func call(t *testing.T, h http.Handler, cookie *http.Cookie, method, path, body string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	var out map[string]any
	if rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("%s %s: bad JSON response %q", method, path, rec.Body.String())
		}
	}
	return rec.Code, out
}

func TestTripsAPI(t *testing.T) {
	h, alice, bob := setup(t)

	t.Run("config exposes tile url", func(t *testing.T) {
		code, cfg := call(t, h, alice, "GET", "/api/v1/config", "")
		if code != 200 || cfg["tileUrl"] != "https://tiles.test/{z}/{x}/{y}.png" ||
			cfg["mapStyleUrl"] != "https://style.test/style.json" || cfg["language"] != "en" {
			t.Fatalf("config: code = %d %v", code, cfg)
		}
		if code, _ := call(t, h, nil, "GET", "/api/v1/config", ""); code != 401 {
			t.Fatalf("unauthenticated config: code = %d, want 401", code)
		}
	})

	t.Run("requires auth", func(t *testing.T) {
		if code, _ := call(t, h, nil, "GET", "/api/v1/trips", ""); code != 401 {
			t.Fatalf("code = %d, want 401", code)
		}
	})

	// Create a trip as alice.
	code, trip := call(t, h, alice, "POST", "/api/v1/trips",
		`{"title":"Japan 2027","description":"Cherry blossoms","startDate":"2027-03-20","endDate":"2027-04-05"}`)
	if code != 201 {
		t.Fatalf("create trip: code = %d %v", code, trip)
	}
	tripID := trip["id"].(string)
	if trip["status"] != "planning" {
		t.Fatalf("default status = %v, want planning", trip["status"])
	}
	tripPath := "/api/v1/trips/" + tripID

	t.Run("validation", func(t *testing.T) {
		for name, body := range map[string]string{
			"missing title":  `{"description":"no title"}`,
			"bad status":     `{"title":"x","status":"someday"}`,
			"bad date":       `{"title":"x","startDate":"03/20/2027"}`,
			"inverted dates": `{"title":"x","startDate":"2027-04-05","endDate":"2027-03-20"}`,
		} {
			if code, _ := call(t, h, alice, "POST", "/api/v1/trips", body); code != 400 {
				t.Fatalf("%s: code = %d, want 400", name, code)
			}
		}
	})

	t.Run("owner isolation", func(t *testing.T) {
		if code, _ := call(t, h, bob, "GET", tripPath, ""); code != 404 {
			t.Fatalf("bob GET = %d, want 404", code)
		}
		if code, _ := call(t, h, bob, "PATCH", tripPath, `{"title":"hijack"}`); code != 404 {
			t.Fatalf("bob PATCH = %d, want 404", code)
		}
		if code, _ := call(t, h, bob, "DELETE", tripPath, ""); code != 404 {
			t.Fatalf("bob DELETE = %d, want 404", code)
		}
		_, list := call(t, h, bob, "GET", "/api/v1/trips", "")
		if n := len(list["trips"].([]any)); n != 0 {
			t.Fatalf("bob sees %d trips, want 0", n)
		}
	})

	t.Run("patch merges and clears", func(t *testing.T) {
		code, updated := call(t, h, alice, "PATCH", tripPath, `{"status":"active","endDate":""}`)
		if code != 200 {
			t.Fatalf("patch: code = %d %v", code, updated)
		}
		if updated["status"] != "active" || updated["title"] != "Japan 2027" || updated["endDate"] != nil {
			t.Fatalf("patch result: %v", updated)
		}
	})

	// Stops.
	var stopIDs []string
	for _, name := range []string{"Tokyo", "Kyoto", "Osaka"} {
		code, stop := call(t, h, alice, "POST", tripPath+"/stops",
			fmt.Sprintf(`{"name":%q,"lat":35.0,"lon":135.0}`, name))
		if code != 201 {
			t.Fatalf("create stop %s: code = %d %v", name, code, stop)
		}
		stopIDs = append(stopIDs, stop["id"].(string))
	}

	t.Run("stop validation", func(t *testing.T) {
		if code, _ := call(t, h, alice, "POST", tripPath+"/stops", `{"name":"HalfCoord","lat":35.0}`); code != 400 {
			t.Fatalf("lat without lon: code = %d, want 400", code)
		}
		if code, _ := call(t, h, alice, "POST", tripPath+"/stops", `{"name":"Bad","lat":95.0,"lon":0}`); code != 400 {
			t.Fatalf("out-of-range lat: code = %d, want 400", code)
		}
	})

	t.Run("stops are ordered and reorderable", func(t *testing.T) {
		reorder := fmt.Sprintf(`{"ids":[%q,%q,%q]}`, stopIDs[2], stopIDs[0], stopIDs[1])
		if code, _ := call(t, h, alice, "PUT", tripPath+"/stops/order", reorder); code != 204 {
			t.Fatalf("reorder: code = %d, want 204", code)
		}
		_, detail := call(t, h, alice, "GET", tripPath, "")
		stops := detail["stops"].([]any)
		if got := stops[0].(map[string]any)["name"]; got != "Osaka" {
			t.Fatalf("first stop after reorder = %v, want Osaka", got)
		}
		// Not a permutation → 400.
		bad := fmt.Sprintf(`{"ids":[%q]}`, stopIDs[0])
		if code, _ := call(t, h, alice, "PUT", tripPath+"/stops/order", bad); code != 400 {
			t.Fatalf("bad reorder: code = %d, want 400", code)
		}
	})

	t.Run("stop patch keeps unspecified fields", func(t *testing.T) {
		code, updated := call(t, h, alice, "PATCH", tripPath+"/stops/"+stopIDs[0], `{"notes":"ryokan district"}`)
		if code != 200 {
			t.Fatalf("patch stop: code = %d %v", code, updated)
		}
		if updated["name"] != "Tokyo" || updated["notes"] != "ryokan district" || updated["lat"] == nil {
			t.Fatalf("patch stop result: %v", updated)
		}
	})

	// Itinerary items.
	code, item := call(t, h, alice, "POST", tripPath+"/items",
		fmt.Sprintf(`{"title":"Fushimi Inari","day":"2027-03-24","startTime":"09:00","stopId":%q,"category":"activity","costCents":0,"currency":"JPY"}`, stopIDs[1]))
	if code != 201 {
		t.Fatalf("create item: code = %d %v", code, item)
	}
	if item["startTime"] != "09:00" {
		t.Fatalf("startTime = %v, want 09:00", item["startTime"])
	}
	itemID := item["id"].(string)

	t.Run("item validation", func(t *testing.T) {
		for name, body := range map[string]string{
			"missing day":         `{"title":"x"}`,
			"bad time":            `{"title":"x","day":"2027-03-24","startTime":"9am"}`,
			"bad category":        `{"title":"x","day":"2027-03-24","category":"nap"}`,
			"cost, no ccy":        `{"title":"x","day":"2027-03-24","costCents":100}`,
			"negative cost":       `{"title":"x","day":"2027-03-24","costCents":-5,"currency":"USD"}`,
			"foreign stopId":      `{"title":"x","day":"2027-03-24","stopId":"00000000-0000-0000-0000-000000000001"}`,
			"bad endTime":         `{"title":"x","day":"2027-03-24","endTime":"25:99"}`,
			"foreign destination": `{"title":"x","day":"2027-03-24","category":"flight","destinationStopId":"00000000-0000-0000-0000-000000000001"}`,
			"home and stop both":  fmt.Sprintf(`{"title":"x","day":"2027-03-24","category":"flight","originHomeId":"00000000-0000-0000-0000-000000000001","stopId":%q}`, stopIDs[0]),
			"foreign home":        `{"title":"x","day":"2027-03-24","category":"flight","originHomeId":"00000000-0000-0000-0000-000000000001"}`,
		} {
			if code, _ := call(t, h, alice, "POST", tripPath+"/items", body); code != 400 {
				t.Fatalf("%s: code = %d, want 400", name, code)
			}
		}
	})

	t.Run("item patch moves day and clears time", func(t *testing.T) {
		code, updated := call(t, h, alice, "PATCH", tripPath+"/items/"+itemID, `{"day":"2027-03-25","startTime":""}`)
		if code != 200 {
			t.Fatalf("patch item: code = %d %v", code, updated)
		}
		if updated["day"] != "2027-03-25" || updated["startTime"] != nil || updated["title"] != "Fushimi Inari" {
			t.Fatalf("patch item result: %v", updated)
		}
	})

	t.Run("detail embeds stops and items", func(t *testing.T) {
		_, detail := call(t, h, alice, "GET", tripPath, "")
		if len(detail["stops"].([]any)) != 3 || len(detail["items"].([]any)) != 1 {
			t.Fatalf("detail: %d stops, %d items", len(detail["stops"].([]any)), len(detail["items"].([]any)))
		}
	})

	t.Run("items reorder within a day", func(t *testing.T) {
		var ids []string
		for _, title := range []string{"Breakfast", "Museum"} {
			code, it := call(t, h, alice, "POST", tripPath+"/items",
				fmt.Sprintf(`{"title":%q,"day":"2027-03-26"}`, title))
			if code != 201 {
				t.Fatalf("create %s: code = %d", title, code)
			}
			ids = append(ids, it["id"].(string))
		}
		body := fmt.Sprintf(`{"day":"2027-03-26","ids":[%q,%q]}`, ids[1], ids[0])
		if code, _ := call(t, h, alice, "PUT", tripPath+"/items/order", body); code != 204 {
			t.Fatalf("reorder items: code = %d, want 204", code)
		}
		_, detail := call(t, h, alice, "GET", tripPath, "")
		var day26 []string
		for _, raw := range detail["items"].([]any) {
			it := raw.(map[string]any)
			if it["day"] == "2027-03-26" {
				day26 = append(day26, it["title"].(string))
			}
		}
		if len(day26) != 2 || day26[0] != "Museum" {
			t.Fatalf("day order after reorder = %v", day26)
		}
		// Wrong day for those ids → 400.
		wrongDay := fmt.Sprintf(`{"day":"2027-03-25","ids":[%q,%q]}`, ids[0], ids[1])
		if code, _ := call(t, h, alice, "PUT", tripPath+"/items/order", wrongDay); code != 400 {
			t.Fatalf("reorder wrong day: code = %d, want 400", code)
		}
	})

	t.Run("delete cascades", func(t *testing.T) {
		if code, _ := call(t, h, alice, "DELETE", tripPath, ""); code != 204 {
			t.Fatalf("delete trip: code = %d", code)
		}
		if code, _ := call(t, h, alice, "GET", tripPath, ""); code != 404 {
			t.Fatalf("after delete: code = %d, want 404", code)
		}
	})
}
