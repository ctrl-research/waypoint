package server

import (
	"fmt"
	"net/http/httptest"
	"testing"
)

func TestTripSharing(t *testing.T) {
	h, alice, bob := setup(t)

	_, trip := call(t, h, alice, "POST", "/api/v1/trips", `{"title":"Shared roadtrip"}`)
	tripPath := "/api/v1/trips/" + trip["id"].(string)
	if trip["role"] != "owner" {
		t.Fatalf("creator role = %v, want owner", trip["role"])
	}

	t.Run("add member validation", func(t *testing.T) {
		if code, _ := call(t, h, alice, "POST", tripPath+"/members", `{"email":"ghost@example.com","role":"viewer"}`); code != 404 {
			t.Fatalf("unknown email: code = %d, want 404", code)
		}
		if code, _ := call(t, h, alice, "POST", tripPath+"/members", `{"email":"alice@example.com","role":"viewer"}`); code != 400 {
			t.Fatalf("owner as member: code = %d, want 400", code)
		}
		if code, _ := call(t, h, alice, "POST", tripPath+"/members", `{"email":"bob@example.com","role":"admin"}`); code != 400 {
			t.Fatalf("bad role: code = %d, want 400", code)
		}
	})

	// Alice shares with bob as viewer.
	code, member := call(t, h, alice, "POST", tripPath+"/members", `{"email":"bob@example.com","role":"viewer"}`)
	if code != 201 {
		t.Fatalf("add member: code = %d %v", code, member)
	}
	bobID := member["userId"].(string)

	t.Run("viewer can read, not write", func(t *testing.T) {
		code, detail := call(t, h, bob, "GET", tripPath, "")
		if code != 200 || detail["trip"].(map[string]any)["role"] != "viewer" {
			t.Fatalf("bob GET: code = %d role = %v", code, detail["trip"].(map[string]any)["role"])
		}
		_, list := call(t, h, bob, "GET", "/api/v1/trips", "")
		if n := len(list["trips"].([]any)); n != 1 {
			t.Fatalf("bob sees %d trips, want 1", n)
		}
		if code, _ := call(t, h, bob, "GET", tripPath+"/journal", ""); code != 200 {
			t.Fatalf("bob journal list: code = %d, want 200", code)
		}
		if code, _ := call(t, h, bob, "PATCH", tripPath, `{"title":"hijack"}`); code != 403 {
			t.Fatalf("viewer PATCH trip: code = %d, want 403", code)
		}
		if code, _ := call(t, h, bob, "POST", tripPath+"/stops", `{"name":"Sneaky"}`); code != 403 {
			t.Fatalf("viewer create stop: code = %d, want 403", code)
		}
		if code, _ := call(t, h, bob, "POST", tripPath+"/members", `{"email":"alice@example.com","role":"viewer"}`); code != 403 {
			t.Fatalf("viewer manage members: code = %d, want 403", code)
		}
	})

	t.Run("member can see photos", func(t *testing.T) {
		_, entry := call(t, h, alice, "POST", tripPath+"/journal", `{"entryDate":"2026-09-01"}`)
		_, photo := uploadPhoto(t, h, alice, tripPath+"/journal/"+entry["id"].(string)+"/photos", tinyPNG, "")
		req := httptest.NewRequest("GET", photo["url"].(string), nil)
		req.AddCookie(bob)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != 200 {
			t.Fatalf("member photo fetch: code = %d, want 200", rec.Code)
		}
	})

	t.Run("editor can write, not administer", func(t *testing.T) {
		if code, _ := call(t, h, alice, "POST", tripPath+"/members", `{"email":"bob@example.com","role":"editor"}`); code != 201 {
			t.Fatalf("promote to editor: code = %d", code)
		}
		if code, _ := call(t, h, bob, "POST", tripPath+"/stops", `{"name":"Detour"}`); code != 201 {
			t.Fatalf("editor create stop: code = %d, want 201", code)
		}
		if code, _ := call(t, h, bob, "PATCH", tripPath, `{"description":"co-planned"}`); code != 200 {
			t.Fatalf("editor PATCH trip: code = %d, want 200", code)
		}
		if code, _ := call(t, h, bob, "DELETE", tripPath, ""); code != 403 {
			t.Fatalf("editor DELETE trip: code = %d, want 403", code)
		}
	})

	t.Run("member list visible to members", func(t *testing.T) {
		code, members := call(t, h, bob, "GET", tripPath+"/members", "")
		if code != 200 || len(members["members"].([]any)) != 1 {
			t.Fatalf("member list: code = %d %v", code, members)
		}
	})

	t.Run("member can leave, owner keeps control", func(t *testing.T) {
		if code, _ := call(t, h, bob, "DELETE", fmt.Sprintf("%s/members/%s", tripPath, bobID), ""); code != 204 {
			t.Fatalf("bob leave: code = %d, want 204", code)
		}
		if code, _ := call(t, h, bob, "GET", tripPath, ""); code != 404 {
			t.Fatalf("bob after leaving: code = %d, want 404", code)
		}
		_, list := call(t, h, bob, "GET", "/api/v1/trips", "")
		if n := len(list["trips"].([]any)); n != 0 {
			t.Fatalf("bob still sees %d trips after leaving", n)
		}
	})
}
