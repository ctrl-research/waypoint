package server

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/ctrl-research/waypoint/internal/auth"
	"github.com/ctrl-research/waypoint/internal/store"
)

// Calendar integration (#52): a per-trip .ics download plus a token-scoped
// subscription feed of every accessible trip, so external calendars stay in
// sync without a session.

func (api *tripsAPI) getCalendarToken(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFrom(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{"token": user.CalendarToken})
}

// createCalendarToken (re)generates the feed token; existing subscriptions
// using the old token stop working, which is also how you rotate a leak.
func (api *tripsAPI) createCalendarToken(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFrom(r.Context())
	buf := make([]byte, 32)
	rand.Read(buf)
	token := base64.RawURLEncoding.EncodeToString(buf)
	if _, err := api.users.SetCalendarToken(r.Context(), user.ID, &token); err != nil {
		apiInternalError(w, "set calendar token", err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"token": token})
}

func (api *tripsAPI) deleteCalendarToken(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFrom(r.Context())
	if _, err := api.users.SetCalendarToken(r.Context(), user.ID, nil); err != nil {
		apiInternalError(w, "clear calendar token", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// serveCalendarFeed streams every accessible trip (and its visible timed
// items) for the token's user. Token-scoped — deliberately no session.
func (api *tripsAPI) serveCalendarFeed(w http.ResponseWriter, r *http.Request) {
	user, err := api.users.ByCalendarToken(r.Context(), r.PathValue("token"))
	if errors.Is(err, store.ErrNotFound) {
		apiError(w, http.StatusNotFound, "not_found", "unknown calendar")
		return
	}
	if err != nil {
		apiInternalError(w, "lookup calendar token", err)
		return
	}

	trips, err := api.trips.ListAccessible(r.Context(), user.ID)
	if err != nil {
		apiInternalError(w, "list trips", err)
		return
	}
	var b strings.Builder
	icsHeader(&b, "Waypoint trips")
	for _, t := range trips {
		icsTripEvents(&b, api.trips, r, t.Trip)
	}
	icsFooter(&b)
	serveICS(w, "waypoint.ics", b.String())
}

// exportICS is the per-trip download next to GPX/GeoJSON/Markdown.
func (api *tripsAPI) exportICS(w http.ResponseWriter, r *http.Request) {
	trip, _, ok := api.tripAccess(w, r, "viewer")
	if !ok {
		return
	}
	var b strings.Builder
	icsHeader(&b, trip.Title)
	icsTripEvents(&b, api.trips, r, trip)
	icsFooter(&b)
	serveICS(w, slugify(trip.Title)+".ics", b.String())
}

// ---- ICS building ------------------------------------------------------------

func icsHeader(b *strings.Builder, name string) {
	b.WriteString("BEGIN:VCALENDAR\r\n")
	b.WriteString("VERSION:2.0\r\n")
	b.WriteString("PRODID:-//Waypoint//EN\r\n")
	b.WriteString("CALSCALE:GREGORIAN\r\n")
	fmt.Fprintf(b, "X-WR-CALNAME:%s\r\n", icsEscape(name))
}

func icsFooter(b *strings.Builder) {
	b.WriteString("END:VCALENDAR\r\n")
}

// icsTripEvents emits the trip's all-day span plus its visible itinerary
// items. Item times are written as floating local time — travel happens in
// the destination's timezone, which the itinerary times already use.
func icsTripEvents(b *strings.Builder, trips *store.Trips, r *http.Request, trip store.Trip) {
	stamp := trip.UpdatedAt.UTC().Format("20060102T150405Z")
	if trip.StartDate != nil {
		end := *trip.StartDate
		if trip.EndDate != nil && !trip.EndDate.Before(*trip.StartDate) {
			end = *trip.EndDate
		}
		b.WriteString("BEGIN:VEVENT\r\n")
		fmt.Fprintf(b, "UID:trip-%s@waypoint\r\n", trip.ID)
		fmt.Fprintf(b, "DTSTAMP:%s\r\n", stamp)
		fmt.Fprintf(b, "DTSTART;VALUE=DATE:%s\r\n", trip.StartDate.Format("20060102"))
		// DTEND is exclusive for all-day events.
		fmt.Fprintf(b, "DTEND;VALUE=DATE:%s\r\n", end.AddDate(0, 0, 1).Format("20060102"))
		fmt.Fprintf(b, "SUMMARY:%s\r\n", icsEscape("✈ "+trip.Title))
		if trip.Description != "" {
			fmt.Fprintf(b, "DESCRIPTION:%s\r\n", icsEscape(trip.Description))
		}
		b.WriteString("TRANSP:TRANSPARENT\r\n")
		b.WriteString("END:VEVENT\r\n")
	}

	items, err := trips.ListVisibleItems(r.Context(), trip.ID)
	if err != nil {
		return // the span event alone still makes a valid calendar
	}
	for _, it := range items {
		b.WriteString("BEGIN:VEVENT\r\n")
		fmt.Fprintf(b, "UID:item-%s@waypoint\r\n", it.ID)
		fmt.Fprintf(b, "DTSTAMP:%s\r\n", stamp)
		day := it.Day.Format("20060102")
		if it.StartTime != "" {
			start := strings.ReplaceAll(it.StartTime, ":", "") + "00"
			fmt.Fprintf(b, "DTSTART:%sT%s\r\n", day, start)
			if it.EndTime != "" {
				endDay := it.Day
				if it.EndTime < it.StartTime { // overnight leg wraps
					endDay = endDay.AddDate(0, 0, 1)
				}
				fmt.Fprintf(b, "DTEND:%sT%s00\r\n", endDay.Format("20060102"), strings.ReplaceAll(it.EndTime, ":", ""))
			}
		} else {
			fmt.Fprintf(b, "DTSTART;VALUE=DATE:%s\r\n", day)
		}
		fmt.Fprintf(b, "SUMMARY:%s\r\n", icsEscape(it.Title))
		if it.Address != "" {
			fmt.Fprintf(b, "LOCATION:%s\r\n", icsEscape(it.Address))
		}
		if it.Notes != "" {
			fmt.Fprintf(b, "DESCRIPTION:%s\r\n", icsEscape(it.Notes))
		}
		b.WriteString("END:VEVENT\r\n")
	}
}

// icsEscape covers RFC 5545 TEXT: backslash, separators, and newlines.
func icsEscape(s string) string {
	r := strings.NewReplacer("\\", "\\\\", ";", "\\;", ",", "\\,", "\r\n", "\\n", "\n", "\\n")
	return r.Replace(s)
}

func serveICS(w http.ResponseWriter, filename, body string) {
	sendDownload(w, filename, "text/calendar; charset=utf-8")
	w.Write([]byte(body))
}
