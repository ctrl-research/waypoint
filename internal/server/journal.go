package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/ctrl-research/waypoint/internal/auth"
	"github.com/ctrl-research/waypoint/internal/photos"
	"github.com/ctrl-research/waypoint/internal/store"
)

// maxPhotoBytes caps a single photo upload (phone photos run 3–12MB).
const maxPhotoBytes = 20 << 20

type entryJSON struct {
	ID        uuid.UUID   `json:"id"`
	EntryDate string      `json:"entryDate"`
	Title     string      `json:"title"`
	Body      string      `json:"body"`
	Lat       *float64    `json:"lat"`
	Lon       *float64    `json:"lon"`
	CreatedAt time.Time   `json:"createdAt"`
	UpdatedAt time.Time   `json:"updatedAt"`
	Photos    []photoJSON `json:"photos"`
}

type photoJSON struct {
	ID          uuid.UUID  `json:"id"`
	URL         string     `json:"url"`
	ContentType string     `json:"contentType"`
	TakenAt     *time.Time `json:"takenAt"`
	Lat         *float64   `json:"lat"`
	Lon         *float64   `json:"lon"`
	Caption     string     `json:"caption"`
}

func toEntryJSON(e store.JournalEntry, photos []photoJSON) entryJSON {
	if photos == nil {
		photos = []photoJSON{}
	}
	return entryJSON{
		ID: e.ID, EntryDate: e.EntryDate.Format(dateFormat), Title: e.Title, Body: e.Body,
		Lat: e.Lat, Lon: e.Lon, CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt, Photos: photos,
	}
}

func toPhotoJSON(p store.JournalPhoto) photoJSON {
	return photoJSON{
		ID: p.ID, URL: "/api/v1/photos/" + p.ID.String(), ContentType: p.ContentType,
		TakenAt: p.TakenAt, Lat: p.Lat, Lon: p.Lon, Caption: p.Caption,
	}
}

// entryRequest is used for POST and PATCH; nil pointers keep current values.
// lat/lon must come together; clearLatLon removes them.
type entryRequest struct {
	EntryDate   *string  `json:"entryDate"`
	Title       *string  `json:"title"`
	Body        *string  `json:"body"`
	Lat         *float64 `json:"lat"`
	Lon         *float64 `json:"lon"`
	ClearLatLon bool     `json:"clearLatLon"`
}

func (req entryRequest) merge(p *store.JournalEntryParams) error {
	if req.EntryDate != nil {
		d, err := time.Parse(dateFormat, *req.EntryDate)
		if err != nil {
			return errors.New("entryDate must be YYYY-MM-DD")
		}
		p.EntryDate = d
	}
	if p.EntryDate.IsZero() {
		return errors.New("entryDate is required")
	}
	if req.Title != nil {
		p.Title = *req.Title
	}
	if req.Body != nil {
		p.Body = *req.Body
	}
	switch {
	case req.ClearLatLon:
		p.Lat, p.Lon = nil, nil
	case req.Lat != nil || req.Lon != nil:
		if req.Lat == nil || req.Lon == nil {
			return errors.New("lat and lon must be provided together")
		}
		if *req.Lat < -90 || *req.Lat > 90 || *req.Lon < -180 || *req.Lon > 180 {
			return errors.New("lat/lon out of range")
		}
		p.Lat, p.Lon = req.Lat, req.Lon
	}
	return nil
}

func (api *tripsAPI) listJournal(w http.ResponseWriter, r *http.Request) {
	trip, _, ok := api.tripAccess(w, r, "viewer")
	if !ok {
		return
	}
	entries, err := api.trips.ListJournalEntries(r.Context(), trip.ID)
	if err != nil {
		apiInternalError(w, "list journal", err)
		return
	}
	allPhotos, err := api.trips.ListJournalPhotosForTrip(r.Context(), trip.ID)
	if err != nil {
		apiInternalError(w, "list journal photos", err)
		return
	}
	byEntry := map[uuid.UUID][]photoJSON{}
	for _, p := range allPhotos {
		byEntry[p.EntryID] = append(byEntry[p.EntryID], toPhotoJSON(p))
	}
	out := make([]entryJSON, 0, len(entries))
	for _, e := range entries {
		out = append(out, toEntryJSON(e, byEntry[e.ID]))
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": out})
}

func (api *tripsAPI) createEntry(w http.ResponseWriter, r *http.Request) {
	trip, ok := api.editableTrip(w, r)
	if !ok {
		return
	}
	user, _ := auth.UserFrom(r.Context())
	var req entryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apiError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}
	var params store.JournalEntryParams
	if err := req.merge(&params); err != nil {
		apiError(w, http.StatusBadRequest, "invalid", err.Error())
		return
	}
	entry, err := api.trips.CreateJournalEntry(r.Context(), trip.ID, user.ID, params)
	if err != nil {
		apiInternalError(w, "create entry", err)
		return
	}
	writeJSON(w, http.StatusCreated, toEntryJSON(entry, nil))
}

func (api *tripsAPI) updateEntry(w http.ResponseWriter, r *http.Request) {
	trip, ok := api.editableTrip(w, r)
	if !ok {
		return
	}
	entryID, err := uuid.Parse(r.PathValue("entryID"))
	if err != nil {
		apiError(w, http.StatusNotFound, "not_found", "entry not found")
		return
	}
	current, err := api.trips.JournalEntryByID(r.Context(), trip.ID, entryID)
	if errors.Is(err, store.ErrNotFound) {
		apiError(w, http.StatusNotFound, "not_found", "entry not found")
		return
	}
	if err != nil {
		apiInternalError(w, "load entry", err)
		return
	}

	var req entryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apiError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}
	params := store.JournalEntryParams{
		EntryDate: current.EntryDate, Title: current.Title, Body: current.Body,
		Lat: current.Lat, Lon: current.Lon,
	}
	if err := req.merge(&params); err != nil {
		apiError(w, http.StatusBadRequest, "invalid", err.Error())
		return
	}
	updated, err := api.trips.UpdateJournalEntry(r.Context(), trip.ID, entryID, params)
	if err != nil {
		apiInternalError(w, "update entry", err)
		return
	}
	photosOut, err := api.entryPhotos(r, entryID)
	if err != nil {
		apiInternalError(w, "list entry photos", err)
		return
	}
	writeJSON(w, http.StatusOK, toEntryJSON(updated, photosOut))
}

func (api *tripsAPI) deleteEntry(w http.ResponseWriter, r *http.Request) {
	trip, ok := api.editableTrip(w, r)
	if !ok {
		return
	}
	entryID, err := uuid.Parse(r.PathValue("entryID"))
	if err != nil {
		apiError(w, http.StatusNotFound, "not_found", "entry not found")
		return
	}
	// Capture file paths before the cascade removes the rows.
	entryPhotos, err := api.trips.ListJournalPhotosForEntry(r.Context(), entryID)
	if err != nil {
		apiInternalError(w, "list entry photos", err)
		return
	}
	switch err := api.trips.DeleteJournalEntry(r.Context(), trip.ID, entryID); {
	case errors.Is(err, store.ErrNotFound):
		apiError(w, http.StatusNotFound, "not_found", "entry not found")
	case err != nil:
		apiInternalError(w, "delete entry", err)
	default:
		for _, p := range entryPhotos {
			_ = api.photos.Remove(p.FilePath)
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func (api *tripsAPI) entryPhotos(r *http.Request, entryID uuid.UUID) ([]photoJSON, error) {
	rows, err := api.trips.ListJournalPhotosForEntry(r.Context(), entryID)
	if err != nil {
		return nil, err
	}
	out := make([]photoJSON, 0, len(rows))
	for _, p := range rows {
		out = append(out, toPhotoJSON(p))
	}
	return out, nil
}

// uploadPhoto accepts a multipart form with a "photo" file field and an
// optional "caption". EXIF timestamp/GPS are extracted when present (#16).
func (api *tripsAPI) uploadPhoto(w http.ResponseWriter, r *http.Request) {
	trip, ok := api.editableTrip(w, r)
	if !ok {
		return
	}
	entryID, err := uuid.Parse(r.PathValue("entryID"))
	if err != nil {
		apiError(w, http.StatusNotFound, "not_found", "entry not found")
		return
	}
	if _, err := api.trips.JournalEntryByID(r.Context(), trip.ID, entryID); err != nil {
		apiError(w, http.StatusNotFound, "not_found", "entry not found")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxPhotoBytes)
	if err := r.ParseMultipartForm(maxPhotoBytes); err != nil {
		apiError(w, http.StatusRequestEntityTooLarge, "too_large", "photo exceeds the 20MB limit")
		return
	}
	file, _, err := r.FormFile("photo")
	if err != nil {
		apiError(w, http.StatusBadRequest, "bad_request", `multipart field "photo" is required`)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		apiInternalError(w, "read upload", err)
		return
	}
	contentType, err := photos.SniffContentType(data)
	if err != nil {
		apiError(w, http.StatusBadRequest, "invalid", err.Error())
		return
	}

	meta := photos.ExtractMeta(bytes.NewReader(data))

	photoID := uuid.New()
	relPath, size, err := api.photos.Save(trip.ID, photoID, contentType, bytes.NewReader(data))
	if err != nil {
		apiInternalError(w, "save photo", err)
		return
	}

	photo, err := api.trips.CreateJournalPhoto(r.Context(), photoID, entryID, store.JournalPhotoParams{
		FilePath: relPath, ContentType: contentType, SizeBytes: size,
		TakenAt: meta.TakenAt, Lat: meta.Lat, Lon: meta.Lon,
		Caption: r.FormValue("caption"),
	})
	if err != nil {
		_ = api.photos.Remove(relPath)
		apiInternalError(w, "create photo", err)
		return
	}
	writeJSON(w, http.StatusCreated, toPhotoJSON(photo))
}

func (api *tripsAPI) deletePhoto(w http.ResponseWriter, r *http.Request) {
	trip, ok := api.editableTrip(w, r)
	if !ok {
		return
	}
	photoID, err := uuid.Parse(r.PathValue("photoID"))
	if err != nil {
		apiError(w, http.StatusNotFound, "not_found", "photo not found")
		return
	}
	photo, err := api.trips.DeleteJournalPhoto(r.Context(), trip.ID, photoID)
	if errors.Is(err, store.ErrNotFound) {
		apiError(w, http.StatusNotFound, "not_found", "photo not found")
		return
	}
	if err != nil {
		apiInternalError(w, "delete photo", err)
		return
	}
	_ = api.photos.Remove(photo.FilePath)
	w.WriteHeader(http.StatusNoContent)
}

// servePhoto streams a stored photo to anyone with a role on its trip.
func (api *tripsAPI) servePhoto(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFrom(r.Context())
	photoID, err := uuid.Parse(r.PathValue("photoID"))
	if err != nil {
		apiError(w, http.StatusNotFound, "not_found", "photo not found")
		return
	}
	photo, err := api.trips.JournalPhotoWithTrip(r.Context(), photoID)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		apiInternalError(w, "load photo", err)
		return
	}
	role := ""
	if err == nil {
		if _, role, err = api.trips.WithRole(r.Context(), photo.PhotoTripID, user.ID); err != nil && !errors.Is(err, store.ErrNotFound) {
			apiInternalError(w, "check access", err)
			return
		}
	}
	if role == "" {
		apiError(w, http.StatusNotFound, "not_found", "photo not found")
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

