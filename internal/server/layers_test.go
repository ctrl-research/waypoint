package server

import (
	"fmt"
	"testing"
)

// TestItineraryLayers covers #73 slice 2: proposal layers, the viewer
// carve-out (members edit their own layer regardless of role), and
// promotion to Final.
func TestItineraryLayers(t *testing.T) {
	h, alice, bob := setup(t)

	_, trip := call(t, h, alice, "POST", "/api/v1/trips", `{"title":"Layered trip"}`)
	tripPath := "/api/v1/trips/" + trip["id"].(string)

	// Alice's first item lazily creates the Final layer.
	code, finalItem := call(t, h, alice, "POST", tripPath+"/items", `{"title":"Museum","day":"2027-05-02"}`)
	if code != 201 {
		t.Fatalf("alice create item: code = %d %v", code, finalItem)
	}
	finalLayerID := finalItem["layerId"].(string)

	if code, _ := call(t, h, alice, "POST", tripPath+"/members", `{"email":"bob@example.com","role":"viewer"}`); code != 201 {
		t.Fatalf("add bob: code = %d", code)
	}

	var bobLayerID string
	t.Run("proposal layer is created once", func(t *testing.T) {
		code, layer := call(t, h, bob, "POST", tripPath+"/layers", "")
		if code != 201 {
			t.Fatalf("ensure layer: code = %d %v", code, layer)
		}
		if layer["ownerId"] == nil || layer["name"] != "Proposal" || layer["color"] != "#d97706" {
			t.Fatalf("layer = %v", layer)
		}
		bobLayerID = layer["id"].(string)

		code, again := call(t, h, bob, "POST", tripPath+"/layers", "")
		if code != 200 || again["id"] != bobLayerID {
			t.Fatalf("second ensure: code = %d id = %v, want 200 %s", code, again["id"], bobLayerID)
		}
	})

	t.Run("viewer edits only their own layer", func(t *testing.T) {
		// No layerId → Final → forbidden for a viewer.
		if code, _ := call(t, h, bob, "POST", tripPath+"/items", `{"title":"Sneaky","day":"2027-05-02"}`); code != 403 {
			t.Fatalf("viewer create on Final: code = %d, want 403", code)
		}
		if code, _ := call(t, h, bob, "PATCH", tripPath+"/items/"+finalItem["id"].(string), `{"title":"hijack"}`); code != 403 {
			t.Fatalf("viewer patch Final item: code = %d, want 403", code)
		}
		if code, _ := call(t, h, bob, "DELETE", tripPath+"/items/"+finalItem["id"].(string), ""); code != 403 {
			t.Fatalf("viewer delete Final item: code = %d, want 403", code)
		}
	})

	// Bob drafts two items on his own layer.
	var bobItems []string
	for _, title := range []string{"Ramen bar", "Jazz club"} {
		code, it := call(t, h, bob, "POST", tripPath+"/items",
			fmt.Sprintf(`{"title":%q,"day":"2027-05-02","layerId":%q}`, title, bobLayerID))
		if code != 201 {
			t.Fatalf("bob create %s: code = %d %v", title, code, it)
		}
		bobItems = append(bobItems, it["id"].(string))
	}

	t.Run("reorder is layer-scoped", func(t *testing.T) {
		body := fmt.Sprintf(`{"day":"2027-05-02","layerId":%q,"ids":[%q,%q]}`, bobLayerID, bobItems[1], bobItems[0])
		if code, _ := call(t, h, bob, "PUT", tripPath+"/items/order", body); code != 204 {
			t.Fatalf("bob reorder own layer: code = %d, want 204", code)
		}
		// Final (implicit layer) is off-limits to a viewer.
		finalOrder := fmt.Sprintf(`{"day":"2027-05-02","ids":[%q]}`, finalItem["id"])
		if code, _ := call(t, h, bob, "PUT", tripPath+"/items/order", finalOrder); code != 403 {
			t.Fatalf("bob reorder Final: code = %d, want 403", code)
		}
	})

	t.Run("promotion moves the item to Final", func(t *testing.T) {
		promote := fmt.Sprintf(`{"layerId":%q}`, finalLayerID)
		if code, _ := call(t, h, bob, "PATCH", tripPath+"/items/"+bobItems[0], promote); code != 403 {
			t.Fatalf("viewer promotes to Final: code = %d, want 403", code)
		}
		code, moved := call(t, h, alice, "PATCH", tripPath+"/items/"+bobItems[0], promote)
		if code != 200 || moved["layerId"] != finalLayerID {
			t.Fatalf("alice promote: code = %d layerId = %v", code, moved["layerId"])
		}
	})

	t.Run("shares expose only the Final layer", func(t *testing.T) {
		_, share := call(t, h, alice, "POST", tripPath+"/shares", "")
		_, pub := call(t, h, nil, "GET", "/api/v1/public/"+share["token"].(string), "")
		if n := len(pub["items"].([]any)); n != 2 {
			t.Fatalf("public items = %d, want 2 (Final only)", n)
		}
	})

	t.Run("layer management", func(t *testing.T) {
		if code, _ := call(t, h, bob, "PATCH", tripPath+"/layers/"+bobLayerID, `{"color":"teal"}`); code != 400 {
			t.Fatalf("bad color: code = %d, want 400", code)
		}
		code, updated := call(t, h, bob, "PATCH", tripPath+"/layers/"+bobLayerID, `{"name":"Bob's picks","color":"#0891b2"}`)
		if code != 200 || updated["color"] != "#0891b2" {
			t.Fatalf("bob recolor: code = %d %v", code, updated)
		}
		if code, _ := call(t, h, bob, "PATCH", tripPath+"/layers/"+finalLayerID, `{"name":"Nope"}`); code != 403 {
			t.Fatalf("viewer renames Final: code = %d, want 403", code)
		}
		if code, _ := call(t, h, alice, "DELETE", tripPath+"/layers/"+finalLayerID, ""); code != 400 {
			t.Fatalf("delete Final: code = %d, want 400", code)
		}
		if code, _ := call(t, h, bob, "DELETE", tripPath+"/layers/"+bobLayerID, ""); code != 204 {
			t.Fatalf("bob delete own layer: code = %d, want 204", code)
		}
		// The layer's remaining item went with it; the promoted one survived.
		_, detail := call(t, h, alice, "GET", tripPath, "")
		if n := len(detail["items"].([]any)); n != 2 {
			t.Fatalf("items after layer delete = %d, want 2", n)
		}
		if n := len(detail["layers"].([]any)); n != 1 {
			t.Fatalf("layers after delete = %d, want 1", n)
		}
	})
}
