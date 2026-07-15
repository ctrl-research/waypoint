// Package geocode proxies place search to a Nominatim instance. The endpoint
// is configurable (WAYPOINT_NOMINATIM_URL); the default is the public
// OpenStreetMap instance, whose usage policy requires an identifying
// User-Agent and at most one request per second — hence the rate limiter.
package geocode

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/time/rate"
)

const userAgent = "waypoint/0.1 (self-hosted travel planner; +https://github.com/ctrl-research/waypoint)"

type Result struct {
	Name string  `json:"name"`
	Lat  float64 `json:"lat"`
	Lon  float64 `json:"lon"`
}

type Client struct {
	baseURL string
	// language is sent as accept-language so results come back in the
	// instance's preferred language ("" keeps Nominatim's native names).
	language string
	http     *http.Client
	limiter  *rate.Limiter
}

func New(baseURL, language string) *Client {
	return &Client{
		baseURL:  baseURL,
		language: language,
		http:     &http.Client{Timeout: 5 * time.Second},
		limiter:  rate.NewLimiter(rate.Limit(1), 1),
	}
}

// Station-/airport-shaped OSM classes. Nominatim has no server-side filter
// for these (its poi layer buries stations under identically-named bars and
// bus stops), so Search fetches deep and keeps only these category/type
// pairs.
var kindClasses = map[string]map[string]bool{
	"station": {
		"railway/station":          true,
		"railway/halt":             true,
		"building/train_station":   true,
		"public_transport/station": true,
		"amenity/bus_station":      true,
	},
	"airport": {
		"aeroway/aerodrome": true,
	},
}

// Search geocodes q, optionally scoped by kind: "city" restricts to
// inhabited places (Nominatim featureType=settlement); "station" and
// "airport" post-filter a deeper result set to matching OSM classes. It
// waits for the rate limiter (bounded by ctx), so bursts of autocomplete
// traffic queue instead of violating the OSM policy.
func (c *Client) Search(ctx context.Context, q string, limit int, kind string) ([]Result, error) {
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, err
	}

	classes := kindClasses[kind]
	fetchLimit := limit
	if classes != nil {
		// The wanted classes often rank below footways and entrances that
		// share the station's name; fetch deep and filter. A type hint in
		// the query pulls the right objects into range ("haneda" alone
		// ranks suburbs; "haneda airport" leads with the aerodrome).
		fetchLimit = 30
		hint := map[string]string{"station": "station", "airport": "airport"}[kind]
		if !strings.Contains(strings.ToLower(q), hint) {
			q += " " + hint
		}
	}
	u := fmt.Sprintf("%s/search?format=jsonv2&limit=%d&q=%s", c.baseURL, fetchLimit, url.QueryEscape(q))
	if kind == "city" {
		u += "&featureType=settlement"
	}
	if c.language != "" {
		u += "&accept-language=" + url.QueryEscape(c.language)
	}
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)

	res, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("nominatim returned %d", res.StatusCode)
	}

	var raw []struct {
		DisplayName string `json:"display_name"`
		Lat         string `json:"lat"`
		Lon         string `json:"lon"`
		Category    string `json:"category"`
		Type        string `json:"type"`
	}
	if err := json.NewDecoder(res.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode nominatim response: %w", err)
	}

	parse := func(matching bool) []Result {
		out := make([]Result, 0, limit)
		for _, r := range raw {
			if matching && classes != nil && !classes[r.Category+"/"+r.Type] {
				continue
			}
			lat, latErr := strconv.ParseFloat(r.Lat, 64)
			lon, lonErr := strconv.ParseFloat(r.Lon, 64)
			if latErr != nil || lonErr != nil {
				continue
			}
			out = append(out, Result{Name: r.DisplayName, Lat: lat, Lon: lon})
			if len(out) >= limit {
				break
			}
		}
		return out
	}
	results := parse(true)
	if len(results) == 0 && classes != nil {
		// Nominatim sometimes ranks only entrances/footways for a station
		// query (common for Japanese stations searched in English). Better
		// to offer those — they carry the right coordinates — than nothing.
		results = parse(false)
	}
	return results, nil
}
