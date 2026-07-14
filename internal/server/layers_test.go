package server

import (
	"fmt"
	"testing"
)

// TestItineraryLayers covers #73: named member layers, the viewer
// carve-out (members edit their own layers regardless of role), and
// compiling items into the shared Plan layer.
func TestItineraryLayers(t *testing.T) {
	h, alice, bob := setup(t)

	_, trip := call(t, h, alice, "POST", "/api/v1/trips", `{"title":"Layered trip"}`)
	tripPath := "/api/v1/trips/" + trip["id"].(string)

	// Alice's first item lazily creates the Plan layer.
	code, planItem := call(t, h, alice, "POST", tripPath+"/items", `{"title":"Museum","day":"2027-05-02"}`)
	if code != 201 {
		t.Fatalf("alice create item: code = %d %v", code, planItem)
	}
	planLayerID := planItem["layerId"].(string)

	if code, _ := call(t, h, alice, "POST", tripPath+"/members", `{"email":"bob@example.com","role":"viewer"}`); code != 201 {
		t.Fatalf("add bob: code = %d", code)
	}

	var bobLayerID string
	t.Run("members create named layers freely", func(t *testing.T) {
		if code, _ := call(t, h, bob, "POST", tripPath+"/layers", `{"name":"  "}`); code != 400 {
			t.Fatalf("blank name: code = %d, want 400", code)
		}
		code, layer := call(t, h, bob, "POST", tripPath+"/layers", `{"name":"Food ideas"}`)
		if code != 201 {
			t.Fatalf("create layer: code = %d %v", code, layer)
		}
		if layer["ownerId"] == nil || layer["name"] != "Food ideas" || layer["color"] != "#d97706" {
			t.Fatalf("layer = %v", layer)
		}
		bobLayerID = layer["id"].(string)

		// A second layer for the same member is fine now.
		code, second := call(t, h, bob, "POST", tripPath+"/layers", `{"name":"Rainy day","color":"#0891b2"}`)
		if code != 201 || second["id"] == bobLayerID || second["color"] != "#0891b2" {
			t.Fatalf("second layer: code = %d %v", code, second)
		}
		if code, _ := call(t, h, bob, "DELETE", tripPath+"/layers/"+second["id"].(string), ""); code != 204 {
			t.Fatalf("delete second layer: code = %d", code)
		}
	})

	t.Run("viewer edits only their own layers", func(t *testing.T) {
		// No layerId → Plan → forbidden for a viewer.
		if code, _ := call(t, h, bob, "POST", tripPath+"/items", `{"title":"Sneaky","day":"2027-05-02"}`); code != 403 {
			t.Fatalf("viewer create on Plan: code = %d, want 403", code)
		}
		if code, _ := call(t, h, bob, "PATCH", tripPath+"/items/"+planItem["id"].(string), `{"title":"hijack"}`); code != 403 {
			t.Fatalf("viewer patch Plan item: code = %d, want 403", code)
		}
		if code, _ := call(t, h, bob, "DELETE", tripPath+"/items/"+planItem["id"].(string), ""); code != 403 {
			t.Fatalf("viewer delete Plan item: code = %d, want 403", code)
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
		// Plan (the implicit layer) is off-limits to a viewer.
		planOrder := fmt.Sprintf(`{"day":"2027-05-02","ids":[%q]}`, planItem["id"])
		if code, _ := call(t, h, bob, "PUT", tripPath+"/items/order", planOrder); code != 403 {
			t.Fatalf("bob reorder Plan: code = %d, want 403", code)
		}
	})

	t.Run("promotion compiles the item into the Plan", func(t *testing.T) {
		promote := fmt.Sprintf(`{"layerId":%q}`, planLayerID)
		if code, _ := call(t, h, bob, "PATCH", tripPath+"/items/"+bobItems[0], promote); code != 403 {
			t.Fatalf("viewer promotes to Plan: code = %d, want 403", code)
		}
		code, moved := call(t, h, alice, "PATCH", tripPath+"/items/"+bobItems[0], promote)
		if code != 200 || moved["layerId"] != planLayerID {
			t.Fatalf("alice promote: code = %d layerId = %v", code, moved["layerId"])
		}
	})

	t.Run("editing keeps another member's home on a leg", func(t *testing.T) {
		// Bob's flight departs from his home; alice may still edit other
		// fields without owning (or resending) that home.
		_, home := call(t, h, bob, "POST", "/api/v1/homes", `{"name":"Bob's place","lat":43.6,"lon":-79.4}`)
		code, leg := call(t, h, bob, "POST", tripPath+"/items",
			fmt.Sprintf(`{"title":"AC123","day":"2027-05-03","category":"flight","originHomeId":%q,"layerId":%q}`, home["id"], bobLayerID))
		if code != 201 {
			t.Fatalf("bob create leg: code = %d %v", code, leg)
		}
		code, updated := call(t, h, alice, "PATCH", tripPath+"/items/"+leg["id"].(string), `{"title":"AC124"}`)
		if code != 200 || updated["originHomeId"] != home["id"] {
			t.Fatalf("alice edit leg: code = %d originHomeId = %v", code, updated["originHomeId"])
		}
		// Setting a home you don't own still fails.
		if code, _ := call(t, h, alice, "PATCH", tripPath+"/items/"+leg["id"].(string),
			fmt.Sprintf(`{"destinationHomeId":%q}`, home["id"])); code != 400 {
			t.Fatalf("alice sets bob's home: code = %d, want 400", code)
		}
		if code, _ := call(t, h, bob, "DELETE", tripPath+"/items/"+leg["id"].(string), ""); code != 204 {
			t.Fatalf("cleanup leg: code = %d", code)
		}
	})

	t.Run("shares expose only the Plan layer", func(t *testing.T) {
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
		if code, _ := call(t, h, bob, "PATCH", tripPath+"/layers/"+planLayerID, `{"name":"Nope"}`); code != 403 {
			t.Fatalf("viewer renames Plan: code = %d, want 403", code)
		}
		if code, _ := call(t, h, alice, "DELETE", tripPath+"/layers/"+planLayerID, ""); code != 400 {
			t.Fatalf("delete Plan: code = %d, want 400", code)
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
