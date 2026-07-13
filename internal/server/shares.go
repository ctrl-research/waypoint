package server

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/ctrl-research/waypoint/internal/store"
)

type shareJSON struct {
	ID        uuid.UUID `json:"id"`
	Token     string    `json:"token"`
	URL       string    `json:"url"`
	CreatedAt time.Time `json:"createdAt"`
}

func toShareJSON(l store.ShareLink) shareJSON {
	return shareJSON{ID: l.ID, Token: l.Token, URL: "/share/" + l.Token, CreatedAt: l.CreatedAt}
}

func (api *tripsAPI) listShares(w http.ResponseWriter, r *http.Request) {
	trip, _, ok := api.tripAccess(w, r, "owner")
	if !ok {
		return
	}
	links, err := api.trips.ListShareLinks(r.Context(), trip.ID)
	if err != nil {
		apiInternalError(w, "list share links", err)
		return
	}
	out := make([]shareJSON, 0, len(links))
	for _, l := range links {
		out = append(out, toShareJSON(l))
	}
	writeJSON(w, http.StatusOK, map[string]any{"shares": out})
}

func (api *tripsAPI) createShare(w http.ResponseWriter, r *http.Request) {
	trip, _, ok := api.tripAccess(w, r, "owner")
	if !ok {
		return
	}
	buf := make([]byte, 32)
	rand.Read(buf)
	link, err := api.trips.CreateShareLink(r.Context(), trip.ID, base64.RawURLEncoding.EncodeToString(buf))
	if err != nil {
		apiInternalError(w, "create share link", err)
		return
	}
	writeJSON(w, http.StatusCreated, toShareJSON(link))
}

func (api *tripsAPI) revokeShare(w http.ResponseWriter, r *http.Request) {
	trip, _, ok := api.tripAccess(w, r, "owner")
	if !ok {
		return
	}
	shareID, err := uuid.Parse(r.PathValue("shareID"))
	if err != nil {
		apiError(w, http.StatusNotFound, "not_found", "share link not found")
		return
	}
	switch err := api.trips.DeleteShareLink(r.Context(), trip.ID, shareID); {
	case errors.Is(err, store.ErrNotFound):
		apiError(w, http.StatusNotFound, "not_found", "share link not found")
	case err != nil:
		apiInternalError(w, "revoke share link", err)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

// ---- public (unauthenticated, token-scoped) ---------------------------------

// publicTrip resolves the share token or writes a 404.
func (api *tripsAPI) publicTrip(w http.ResponseWriter, r *http.Request) (store.Trip, bool) {
	token := r.PathValue("token")
	if len(token) < 20 {
		apiError(w, http.StatusNotFound, "not_found", "unknown share link")
		return store.Trip{}, false
	}
	trip, err := api.trips.TripByShareToken(r.Context(), token)
	if errors.Is(err, store.ErrNotFound) {
		apiError(w, http.StatusNotFound, "not_found", "unknown share link")
		return store.Trip{}, false
	}
	if err != nil {
		apiInternalError(w, "resolve share token", err)
		return store.Trip{}, false
	}
	return trip, true
}

// servePublicTrip returns the whole read-only trip payload: trip, stops,
// itinerary, and journal entries with photo URLs scoped to this token.
func (api *tripsAPI) servePublicTrip(w http.ResponseWriter, r *http.Request) {
	trip, ok := api.publicTrip(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	token := r.PathValue("token")

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
	allPhotos, err := api.trips.ListJournalPhotosForTrip(ctx, trip.ID)
	if err != nil {
		apiInternalError(w, "list photos", err)
		return
	}

	photosByEntry := map[uuid.UUID][]photoJSON{}
	for _, p := range allPhotos {
		pj := toPhotoJSON(p)
		pj.URL = "/api/v1/public/" + token + "/photos/" + p.ID.String()
		photosByEntry[p.EntryID] = append(photosByEntry[p.EntryID], pj)
	}

	stopsOut := make([]stopJSON, 0, len(stops))
	for _, s := range stops {
		stopsOut = append(stopsOut, toStopJSON(s))
	}
	itemsOut := make([]itemJSON, 0, len(items))
	for _, it := range items {
		itemsOut = append(itemsOut, toItemJSON(it))
	}
	entriesOut := make([]entryJSON, 0, len(entries))
	for _, e := range entries {
		entriesOut = append(entriesOut, toEntryJSON(e, photosByEntry[e.ID]))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"trip":        toTripJSON(trip, "viewer"),
		"stops":       stopsOut,
		"items":       itemsOut,
		"entries":     entriesOut,
		"tileUrl":     api.opts.TileURL,
		"mapStyleUrl": api.opts.MapStyleURL,
		"language":    api.opts.Language,
	})
}

// servePublicPhoto streams a photo when it belongs to the token's trip.
func (api *tripsAPI) servePublicPhoto(w http.ResponseWriter, r *http.Request) {
	trip, ok := api.publicTrip(w, r)
	if !ok {
		return
	}
	photoID, err := uuid.Parse(r.PathValue("photoID"))
	if err != nil {
		apiError(w, http.StatusNotFound, "not_found", "photo not found")
		return
	}
	photo, err := api.trips.JournalPhotoWithTrip(r.Context(), photoID)
	if errors.Is(err, store.ErrNotFound) || (err == nil && photo.PhotoTripID != trip.ID) {
		apiError(w, http.StatusNotFound, "not_found", "photo not found")
		return
	}
	if err != nil {
		apiInternalError(w, "load photo", err)
		return
	}
	f, err := api.photos.Open(photo.FilePath)
	if err != nil {
		apiInternalError(w, "open photo", err)
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", photo.ContentType)
	w.Header().Set("Cache-Control", "private, max-age=86400")
	http.ServeContent(w, r, "", photo.CreatedAt, f)
}
