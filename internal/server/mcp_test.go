package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// bearerTransport stamps the MCP token on every request.
type bearerTransport struct{ token string }

func (t *bearerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+t.token)
	return http.DefaultTransport.RoundTrip(req)
}

func TestMCPServer(t *testing.T) {
	h, alice, _ := setup(t)
	srv := httptest.NewServer(h)
	defer srv.Close()

	_, created := call(t, h, alice, "POST", "/api/v1/mcp/token", "")
	token := created["token"].(string)

	t.Run("bearer token is required", func(t *testing.T) {
		res, err := http.Post(srv.URL+"/mcp", "application/json", strings.NewReader("{}"))
		if err != nil {
			t.Fatalf("post: %v", err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusUnauthorized {
			t.Fatalf("no token: status = %d, want 401", res.StatusCode)
		}
	})

	ctx := context.Background()
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint:   srv.URL + "/mcp",
		HTTPClient: &http.Client{Transport: &bearerTransport{token}},
	}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer session.Close()

	// callTool returns the structured content as a map, failing on tool errors.
	callTool := func(name string, args map[string]any) map[string]any {
		t.Helper()
		res, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if res.IsError {
			t.Fatalf("%s: tool error: %+v", name, res.Content)
		}
		raw, err := json.Marshal(res.StructuredContent)
		if err != nil {
			t.Fatalf("%s: marshal structured: %v", name, err)
		}
		var out map[string]any
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatalf("%s: structured content %q: %v", name, raw, err)
		}
		return out
	}

	trip := callTool("create_trip", map[string]any{
		"title": "Backfilled Japan", "startDate": "2024-04-01", "endDate": "2024-04-10", "status": "completed",
	})
	if trip["status"] != "completed" || trip["id"] == "" {
		t.Fatalf("create_trip = %v", trip)
	}
	tripID := trip["id"].(string)

	area := callTool("add_area", map[string]any{
		"tripId": tripID, "name": "Kyoto", "lat": 35.0116, "lon": 135.7681,
		"arrivalDate": "2024-04-02", "departureDate": "2024-04-05",
	})
	if area["name"] != "Kyoto" || area["id"] == "" {
		t.Fatalf("add_area = %v", area)
	}

	item := callTool("add_item", map[string]any{
		"tripId": tripID, "title": "Nishiki market", "day": "2024-04-03",
		"category": "food", "areaId": area["id"], "layer": "Ideas",
	})
	if item["layer"] != "Ideas" || item["category"] != "food" {
		t.Fatalf("add_item = %v", item)
	}

	detail := callTool("get_trip", map[string]any{"tripId": tripID})
	if n := len(detail["areas"].([]any)); n != 1 {
		t.Fatalf("get_trip areas = %d, want 1", n)
	}
	if n := len(detail["items"].([]any)); n != 1 {
		t.Fatalf("get_trip items = %d, want 1", n)
	}
	layers := fmt.Sprintf("%v", detail["layers"])
	if !strings.Contains(layers, "Ideas") {
		t.Fatalf("get_trip layers = %v, want Ideas", detail["layers"])
	}

	list := callTool("list_trips", map[string]any{})
	if n := len(list["trips"].([]any)); n != 1 {
		t.Fatalf("list_trips = %d, want 1", n)
	}

	callTool("delete_item", map[string]any{"tripId": tripID, "itemId": item["id"]})
	callTool("delete_area", map[string]any{"tripId": tripID, "areaId": area["id"]})
	detail = callTool("get_trip", map[string]any{"tripId": tripID})
	if len(detail["areas"].([]any)) != 0 || len(detail["items"].([]any)) != 0 {
		t.Fatalf("after deletes: %v", detail)
	}

	// ---- update_trip --------------------------------------------------------

	updatedTrip := callTool("update_trip", map[string]any{
		"tripId": tripID, "title": "Japan 2024", "status": "active",
	})
	if updatedTrip["title"] != "Japan 2024" {
		t.Fatalf("update_trip title = %v, want Japan 2024", updatedTrip["title"])
	}
	if updatedTrip["status"] != "active" {
		t.Fatalf("update_trip status = %v, want active", updatedTrip["status"])
	}
	if updatedTrip["startDate"] != "2024-04-01" {
		t.Fatalf("update_trip startDate changed unexpectedly: %v", updatedTrip["startDate"])
	}

	// ---- update_area --------------------------------------------------------

	area2 := callTool("add_area", map[string]any{
		"tripId": tripID, "name": "Osaka", "lat": 34.6937, "lon": 135.5023,
	})
	updatedArea := callTool("update_area", map[string]any{
		"tripId": tripID, "areaId": area2["id"], "name": "Osaka City",
	})
	if updatedArea["name"] != "Osaka City" {
		t.Fatalf("update_area name = %v, want Osaka City", updatedArea["name"])
	}
	if lat, ok := updatedArea["lat"].(float64); !ok || lat != 34.6937 {
		t.Fatalf("update_area lat changed unexpectedly: %v", updatedArea["lat"])
	}

	// ---- update_item --------------------------------------------------------

	item2 := callTool("add_item", map[string]any{
		"tripId": tripID, "title": "Fushimi Inari", "day": "2024-04-03",
		"category": "activity", "areaId": area2["id"],
	})
	updatedItem := callTool("update_item", map[string]any{
		"tripId": tripID, "itemId": item2["id"], "title": "Fushimi Inari Shrine", "category": "food",
	})
	if updatedItem["title"] != "Fushimi Inari Shrine" {
		t.Fatalf("update_item title = %v, want Fushimi Inari Shrine", updatedItem["title"])
	}
	if updatedItem["category"] != "food" {
		t.Fatalf("update_item category = %v, want food", updatedItem["category"])
	}

	t.Run("rotating the token cuts access", func(t *testing.T) {
		call(t, h, alice, "POST", "/api/v1/mcp/token", "")
		res, err := (&http.Client{Transport: &bearerTransport{token}}).Post(
			srv.URL+"/mcp", "application/json", strings.NewReader("{}"))
		if err != nil {
			t.Fatalf("post: %v", err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusUnauthorized {
			t.Fatalf("old token: status = %d, want 401", res.StatusCode)
		}
	})
}
