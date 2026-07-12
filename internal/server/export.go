package server

import (
	"archive/zip"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ctrl-research/waypoint/internal/store"
)

// Export formats (#25). All are viewer-accessible: if you can see a trip,
// you can take a copy of it.

// ---- GPX --------------------------------------------------------------------

type gpxFile struct {
	XMLName  xml.Name    `xml:"gpx"`
	Version  string      `xml:"version,attr"`
	Creator  string      `xml:"creator,attr"`
	XMLNS    string      `xml:"xmlns,attr"`
	Metadata gpxMetadata `xml:"metadata"`
	Wpts     []gpxPoint  `xml:"wpt"`
	Rte      *gpxRoute   `xml:"rte,omitempty"`
}

type gpxMetadata struct {
	Name string `xml:"name"`
	Desc string `xml:"desc,omitempty"`
}

type gpxPoint struct {
	Lat  float64 `xml:"lat,attr"`
	Lon  float64 `xml:"lon,attr"`
	Name string  `xml:"name,omitempty"`
	Desc string  `xml:"desc,omitempty"`
}

type gpxRoute struct {
	Name string     `xml:"name"`
	Pts  []gpxPoint `xml:"rtept"`
}

func (api *tripsAPI) exportGPX(w http.ResponseWriter, r *http.Request) {
	trip, _, ok := api.tripAccess(w, r, "viewer")
	if !ok {
		return
	}
	stops, err := api.trips.ListStops(r.Context(), trip.ID)
	if err != nil {
		apiInternalError(w, "list stops", err)
		return
	}

	out := gpxFile{
		Version: "1.1", Creator: "waypoint", XMLNS: "http://www.topografix.com/GPX/1/1",
		Metadata: gpxMetadata{Name: trip.Title, Desc: trip.Description},
	}
	var located []gpxPoint
	for _, s := range stops {
		if s.Lat == nil || s.Lon == nil {
			continue
		}
		located = append(located, gpxPoint{Lat: *s.Lat, Lon: *s.Lon, Name: s.Name, Desc: s.Notes})
	}
	out.Wpts = located
	if len(located) > 1 {
		out.Rte = &gpxRoute{Name: trip.Title, Pts: located}
	}

	sendDownload(w, slugify(trip.Title)+".gpx", "application/gpx+xml")
	io.WriteString(w, xml.Header)
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	enc.Encode(out)
}

// ---- GeoJSON ------------------------------------------------------------------

func (api *tripsAPI) exportGeoJSON(w http.ResponseWriter, r *http.Request) {
	trip, _, ok := api.tripAccess(w, r, "viewer")
	if !ok {
		return
	}
	ctx := r.Context()
	stops, err := api.trips.ListStops(ctx, trip.ID)
	if err != nil {
		apiInternalError(w, "list stops", err)
		return
	}
	entries, err := api.trips.ListJournalEntries(ctx, trip.ID)
	if err != nil {
		apiInternalError(w, "list journal", err)
		return
	}

	features := []any{}
	var line [][2]float64
	for _, s := range stops {
		if s.Lat == nil || s.Lon == nil {
			continue
		}
		line = append(line, [2]float64{*s.Lon, *s.Lat})
		features = append(features, geoFeature("Point", []any{*s.Lon, *s.Lat}, map[string]any{
			"type": "stop", "name": s.Name, "notes": s.Notes,
			"arrivalDate": formatDate(s.ArrivalDate), "departureDate": formatDate(s.DepartureDate),
		}))
	}
	if len(line) > 1 {
		features = append(features, geoFeature("LineString", line, map[string]any{
			"type": "route", "name": trip.Title,
		}))
	}
	for _, e := range entries {
		if e.Lat == nil || e.Lon == nil {
			continue
		}
		features = append(features, geoFeature("Point", []any{*e.Lon, *e.Lat}, map[string]any{
			"type": "journal", "title": e.Title, "date": e.EntryDate.Format(dateFormat),
		}))
	}

	sendDownload(w, slugify(trip.Title)+".geojson", "application/geo+json")
	json.NewEncoder(w).Encode(map[string]any{
		"type":     "FeatureCollection",
		"features": features,
	})
}

func geoFeature(geomType string, coords any, props map[string]any) map[string]any {
	return map[string]any{
		"type":       "Feature",
		"geometry":   map[string]any{"type": geomType, "coordinates": coords},
		"properties": props,
	}
}

// ---- Markdown bundle ------------------------------------------------------------

// exportMarkdown streams a zip: trip.md plus the journal photos it references.
func (api *tripsAPI) exportMarkdown(w http.ResponseWriter, r *http.Request) {
	trip, _, ok := api.tripAccess(w, r, "viewer")
	if !ok {
		return
	}
	ctx := r.Context()
	stops, err := api.trips.ListStops(ctx, trip.ID)
	if err != nil {
		apiInternalError(w, "list stops", err)
		return
	}
	items, err := api.trips.ListItems(ctx, trip.ID)
	if err != nil {
		apiInternalError(w, "list items", err)
		return
	}
	entries, err := api.trips.ListJournalEntries(ctx, trip.ID)
	if err != nil {
		apiInternalError(w, "list journal", err)
		return
	}
	photos, err := api.trips.ListJournalPhotosForTrip(ctx, trip.ID)
	if err != nil {
		apiInternalError(w, "list photos", err)
		return
	}
	photosByEntry := map[string][]store.JournalPhoto{}
	for _, p := range photos {
		photosByEntry[p.EntryID.String()] = append(photosByEntry[p.EntryID.String()], p)
	}

	slug := slugify(trip.Title)
	sendDownload(w, slug+".zip", "application/zip")
	zw := zip.NewWriter(w)
	defer zw.Close()

	md, _ := zw.Create(slug + "/trip.md")
	writeTripMarkdown(md, trip, stops, items, entries, photosByEntry)

	for _, p := range photos {
		f, err := api.photos.Open(p.FilePath)
		if err != nil {
			continue // skip missing files rather than corrupting the archive
		}
		dst, _ := zw.Create(slug + "/photos/" + filepath.Base(p.FilePath))
		io.Copy(dst, f)
		f.Close()
	}
}

func writeTripMarkdown(w io.Writer, trip store.Trip, stops []store.Stop, items []store.ItineraryItem,
	entries []store.JournalEntry, photosByEntry map[string][]store.JournalPhoto) {

	fmt.Fprintf(w, "# %s\n\n", trip.Title)
	if trip.StartDate != nil && trip.EndDate != nil {
		fmt.Fprintf(w, "%s – %s · %s\n\n", trip.StartDate.Format(dateFormat), trip.EndDate.Format(dateFormat), trip.Status)
	}
	if trip.Description != "" {
		fmt.Fprintf(w, "%s\n\n", trip.Description)
	}

	if len(stops) > 0 {
		fmt.Fprint(w, "## Stops\n\n")
		for i, s := range stops {
			fmt.Fprintf(w, "%d. **%s**", i+1, s.Name)
			if s.Lat != nil && s.Lon != nil {
				fmt.Fprintf(w, " (%.5f, %.5f)", *s.Lat, *s.Lon)
			}
			if s.Notes != "" {
				fmt.Fprintf(w, " — %s", s.Notes)
			}
			fmt.Fprintln(w)
		}
		fmt.Fprintln(w)
	}

	if len(items) > 0 {
		fmt.Fprint(w, "## Itinerary\n\n")
		lastDay := ""
		for _, it := range items {
			day := it.Day.Format(dateFormat)
			if day != lastDay {
				fmt.Fprintf(w, "### %s\n\n", day)
				lastDay = day
			}
			fmt.Fprint(w, "- ")
			if it.StartTime != "" {
				fmt.Fprintf(w, "%s · ", it.StartTime)
			}
			fmt.Fprintf(w, "%s (%s)", it.Title, it.Category)
			if it.Notes != "" {
				fmt.Fprintf(w, " — %s", it.Notes)
			}
			fmt.Fprintln(w)
		}
		fmt.Fprintln(w)
	}

	if len(entries) > 0 {
		fmt.Fprint(w, "## Journal\n\n")
		for _, e := range entries {
			title := e.Title
			if title == "" {
				title = e.EntryDate.Format(dateFormat)
			} else {
				title = e.EntryDate.Format(dateFormat) + " — " + title
			}
			fmt.Fprintf(w, "### %s\n\n", title)
			if e.Body != "" {
				fmt.Fprintf(w, "%s\n\n", e.Body)
			}
			for _, p := range photosByEntry[e.ID.String()] {
				caption := p.Caption
				if caption == "" {
					caption = "photo"
				}
				fmt.Fprintf(w, "![%s](photos/%s)\n", caption, filepath.Base(p.FilePath))
			}
			fmt.Fprintln(w)
		}
	}
}

// ---- shared -----------------------------------------------------------------

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	slug := strings.Trim(slugRe.ReplaceAllString(strings.ToLower(s), "-"), "-")
	if slug == "" {
		return "trip"
	}
	return slug
}

func sendDownload(w http.ResponseWriter, filename, contentType string) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
}
