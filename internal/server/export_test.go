package server

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestExport(t *testing.T) {
	h, alice, bob := setup(t)

	_, trip := call(t, h, alice, "POST", "/api/v1/trips",
		`{"title":"Alps & Dolomites 2027!","startDate":"2027-06-01","endDate":"2027-06-10"}`)
	tripPath := "/api/v1/trips/" + trip["id"].(string)
	call(t, h, alice, "POST", tripPath+"/stops", `{"name":"Innsbruck","lat":47.27,"lon":11.39}`)
	call(t, h, alice, "POST", tripPath+"/stops", `{"name":"Bolzano","lat":46.5,"lon":11.35}`)
	call(t, h, alice, "POST", tripPath+"/items", `{"title":"Via ferrata","day":"2027-06-03","startTime":"08:00","category":"activity"}`)
	_, entry := call(t, h, alice, "POST", tripPath+"/journal",
		`{"entryDate":"2027-06-03","title":"Cables and clouds","body":"Legs like jelly.","lat":46.5,"lon":11.8}`)
	uploadPhoto(t, h, alice, tripPath+"/journal/"+entry["id"].(string)+"/photos", tinyPNG, "summit")

	fetch := func(path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", path, nil)
		req.AddCookie(alice)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	t.Run("gpx", func(t *testing.T) {
		rec := fetch(tripPath + "/export/gpx")
		if rec.Code != 200 {
			t.Fatalf("code = %d", rec.Code)
		}
		if cd := rec.Header().Get("Content-Disposition"); !strings.Contains(cd, "alps-dolomites-2027.gpx") {
			t.Fatalf("Content-Disposition = %q", cd)
		}
		var parsed struct {
			Wpts []struct {
				Lat  float64 `xml:"lat,attr"`
				Name string  `xml:"name"`
			} `xml:"wpt"`
			Rte struct {
				Pts []struct{} `xml:"rtept"`
			} `xml:"rte"`
		}
		if err := xml.Unmarshal(rec.Body.Bytes(), &parsed); err != nil {
			t.Fatalf("invalid GPX XML: %v", err)
		}
		if len(parsed.Wpts) != 2 || parsed.Wpts[0].Name != "Innsbruck" || len(parsed.Rte.Pts) != 2 {
			t.Fatalf("gpx content: %+v", parsed)
		}
	})

	t.Run("geojson", func(t *testing.T) {
		rec := fetch(tripPath + "/export/geojson")
		if rec.Code != 200 {
			t.Fatalf("code = %d", rec.Code)
		}
		var fc struct {
			Type     string `json:"type"`
			Features []struct {
				Geometry struct {
					Type string `json:"type"`
				} `json:"geometry"`
				Properties map[string]any `json:"properties"`
			} `json:"features"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &fc); err != nil {
			t.Fatalf("invalid GeoJSON: %v", err)
		}
		// 2 stops + 1 route + 1 located journal entry
		if fc.Type != "FeatureCollection" || len(fc.Features) != 4 {
			t.Fatalf("features = %d, want 4", len(fc.Features))
		}
		kinds := map[string]int{}
		for _, f := range fc.Features {
			kinds[f.Properties["type"].(string)]++
		}
		if kinds["stop"] != 2 || kinds["route"] != 1 || kinds["journal"] != 1 {
			t.Fatalf("kinds = %v", kinds)
		}
	})

	t.Run("markdown bundle", func(t *testing.T) {
		rec := fetch(tripPath + "/export/markdown")
		if rec.Code != 200 {
			t.Fatalf("code = %d", rec.Code)
		}
		zr, err := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
		if err != nil {
			t.Fatalf("invalid zip: %v", err)
		}
		var md string
		photoCount := 0
		for _, f := range zr.File {
			if strings.HasSuffix(f.Name, "trip.md") {
				r, _ := f.Open()
				b, _ := io.ReadAll(r)
				md = string(b)
			}
			if strings.Contains(f.Name, "/photos/") {
				photoCount++
			}
		}
		if photoCount != 1 {
			t.Fatalf("photos in zip = %d, want 1", photoCount)
		}
		for _, want := range []string{"# Alps & Dolomites 2027!", "## Stops", "Innsbruck", "## Itinerary", "08:00 · Via ferrata", "## Journal", "Cables and clouds", "![summit](photos/"} {
			if !strings.Contains(md, want) {
				t.Fatalf("trip.md missing %q\n---\n%s", want, md)
			}
		}
	})

	t.Run("viewer can export, outsider cannot", func(t *testing.T) {
		asBob := func() int {
			req := httptest.NewRequest("GET", tripPath+"/export/gpx", nil)
			req.AddCookie(bob)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			return rec.Code
		}
		if code := asBob(); code != 404 {
			t.Fatalf("outsider export: code = %d, want 404", code)
		}
		call(t, h, alice, "POST", tripPath+"/members", `{"email":"bob@example.com","role":"viewer"}`)
		if code := asBob(); code != 200 {
			t.Fatalf("viewer export: code = %d, want 200", code)
		}
	})
}
