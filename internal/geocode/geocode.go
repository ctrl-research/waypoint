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

// Search geocodes q, optionally scoped by kind: "city" restricts to
// inhabited places (Nominatim featureType=settlement) and "station" to the
// railway layer (train/metro stations, OSM-backed). It waits for the rate
// limiter (bounded by ctx), so bursts of autocomplete traffic queue instead
// of violating the OSM policy.
func (c *Client) Search(ctx context.Context, q string, limit int, kind string) ([]Result, error) {
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, err
	}

	u := fmt.Sprintf("%s/search?format=jsonv2&limit=%d&q=%s", c.baseURL, limit, url.QueryEscape(q))
	switch kind {
	case "city":
		u += "&featureType=settlement"
	case "station":
		u += "&layer=railway"
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
	}
	if err := json.NewDecoder(res.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode nominatim response: %w", err)
	}

	results := make([]Result, 0, len(raw))
	for _, r := range raw {
		lat, latErr := strconv.ParseFloat(r.Lat, 64)
		lon, lonErr := strconv.ParseFloat(r.Lon, 64)
		if latErr != nil || lonErr != nil {
			continue
		}
		results = append(results, Result{Name: r.DisplayName, Lat: lat, Lon: lon})
	}
	return results, nil
}
