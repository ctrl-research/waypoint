package server

import (
	"net/http/httptest"
	"testing"
)

func TestShareLinks(t *testing.T) {
	h, alice, bob := setup(t)

	_, trip := call(t, h, alice, "POST", "/api/v1/trips", `{"title":"Public trip","startDate":"2026-10-01"}`)
	tripPath := "/api/v1/trips/" + trip["id"].(string)
	call(t, h, alice, "POST", tripPath+"/stops", `{"name":"Lisbon","lat":38.72,"lon":-9.14}`)
	_, entry := call(t, h, alice, "POST", tripPath+"/journal", `{"entryDate":"2026-10-02","body":"pastel de nata"}`)
	uploadPhoto(t, h, alice, tripPath+"/journal/"+entry["id"].(string)+"/photos", tinyPNG, "")

	t.Run("only the owner manages shares", func(t *testing.T) {
		call(t, h, alice, "POST", tripPath+"/members", `{"email":"bob@example.com","role":"editor"}`)
		if code, _ := call(t, h, bob, "POST", tripPath+"/shares", ""); code != 403 {
			t.Fatalf("editor create share: code = %d, want 403", code)
		}
	})

	code, share := call(t, h, alice, "POST", tripPath+"/shares", "")
	if code != 201 {
		t.Fatalf("create share: code = %d %v", code, share)
	}
	token := share["token"].(string)
	publicPath := "/api/v1/public/" + token

	t.Run("public payload without a session", func(t *testing.T) {
		code, payload := call(t, h, nil, "GET", publicPath, "")
		if code != 200 {
			t.Fatalf("public GET: code = %d", code)
		}
		if payload["trip"].(map[string]any)["title"] != "Public trip" {
			t.Fatalf("payload trip: %v", payload["trip"])
		}
		if len(payload["stops"].([]any)) != 1 || len(payload["entries"].([]any)) != 1 {
			t.Fatalf("payload sizes: %d stops %d entries",
				len(payload["stops"].([]any)), len(payload["entries"].([]any)))
		}
		if payload["tileUrl"] != "https://tiles.test/{z}/{x}/{y}.png" {
			t.Fatalf("tileUrl = %v", payload["tileUrl"])
		}

		entryPhotos := payload["entries"].([]any)[0].(map[string]any)["photos"].([]any)
		photoURL := entryPhotos[0].(map[string]any)["url"].(string)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", photoURL, nil))
		if rec.Code != 200 {
			t.Fatalf("public photo: code = %d (url %s)", rec.Code, photoURL)
		}
	})

	t.Run("wrong or foreign identifiers 404", func(t *testing.T) {
		if code, _ := call(t, h, nil, "GET", "/api/v1/public/definitely-not-a-real-token-here", ""); code != 404 {
			t.Fatalf("bad token: code = %d, want 404", code)
		}
		// A photo from a different trip is not reachable through this token.
		_, otherTrip := call(t, h, alice, "POST", "/api/v1/trips", `{"title":"Private"}`)
		otherPath := "/api/v1/trips/" + otherTrip["id"].(string)
		_, otherEntry := call(t, h, alice, "POST", otherPath+"/journal", `{"entryDate":"2026-10-03"}`)
		_, otherPhoto := uploadPhoto(t, h, alice, otherPath+"/journal/"+otherEntry["id"].(string)+"/photos", tinyPNG, "")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", publicPath+"/photos/"+otherPhoto["id"].(string), nil))
		if rec.Code != 404 {
			t.Fatalf("foreign photo via token: code = %d, want 404", rec.Code)
		}
	})

	t.Run("revoke kills the link", func(t *testing.T) {
		if code, _ := call(t, h, alice, "DELETE", tripPath+"/shares/"+share["id"].(string), ""); code != 204 {
			t.Fatalf("revoke: code = %d", code)
		}
		if code, _ := call(t, h, nil, "GET", publicPath, ""); code != 404 {
			t.Fatalf("public GET after revoke: code = %d, want 404", code)
		}
	})

}
