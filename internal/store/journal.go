package store

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/ctrl-research/waypoint/internal/store/sqlcgen"
)

type (
	JournalEntry = sqlcgen.JournalEntry
	JournalPhoto = sqlcgen.JournalPhoto
)

type JournalEntryParams struct {
	EntryDate time.Time
	Title     string
	Body      string
	Lat       *float64
	Lon       *float64
}

func (s *Trips) CreateJournalEntry(ctx context.Context, tripID, authorID uuid.UUID, p JournalEntryParams) (JournalEntry, error) {
	e, err := s.q.CreateJournalEntry(ctx, sqlcgen.CreateJournalEntryParams{
		TripID: tripID, AuthorID: authorID, EntryDate: p.EntryDate,
		Title: p.Title, Body: p.Body, Lat: p.Lat, Lon: p.Lon,
	})
	if err == nil {
		s.touch(ctx, tripID)
	}
	return e, translate(err)
}

func (s *Trips) ListJournalEntries(ctx context.Context, tripID uuid.UUID) ([]JournalEntry, error) {
	entries, err := s.q.ListJournalEntries(ctx, tripID)
	if entries == nil {
		entries = []JournalEntry{}
	}
	return entries, err
}

func (s *Trips) JournalEntryByID(ctx context.Context, tripID, entryID uuid.UUID) (JournalEntry, error) {
	e, err := s.q.JournalEntryByID(ctx, sqlcgen.JournalEntryByIDParams{TripID: tripID, ID: entryID})
	return e, translate(err)
}

func (s *Trips) UpdateJournalEntry(ctx context.Context, tripID, entryID uuid.UUID, p JournalEntryParams) (JournalEntry, error) {
	e, err := s.q.UpdateJournalEntry(ctx, sqlcgen.UpdateJournalEntryParams{
		TripID: tripID, ID: entryID, EntryDate: p.EntryDate,
		Title: p.Title, Body: p.Body, Lat: p.Lat, Lon: p.Lon,
	})
	if err == nil {
		s.touch(ctx, tripID)
	}
	return e, translate(err)
}

func (s *Trips) DeleteJournalEntry(ctx context.Context, tripID, entryID uuid.UUID) error {
	n, err := s.q.DeleteJournalEntry(ctx, sqlcgen.DeleteJournalEntryParams{TripID: tripID, ID: entryID})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	s.touch(ctx, tripID)
	return nil
}

type JournalPhotoParams struct {
	FilePath    string
	ContentType string
	SizeBytes   int64
	TakenAt     *time.Time
	Lat         *float64
	Lon         *float64
	Caption     string
}

func (s *Trips) CreateJournalPhoto(ctx context.Context, photoID, entryID uuid.UUID, p JournalPhotoParams) (JournalPhoto, error) {
	photo, err := s.q.CreateJournalPhoto(ctx, sqlcgen.CreateJournalPhotoParams{
		ID: photoID, EntryID: entryID, FilePath: p.FilePath, ContentType: p.ContentType, SizeBytes: p.SizeBytes,
		TakenAt: p.TakenAt, Lat: p.Lat, Lon: p.Lon, Caption: p.Caption,
	})
	return photo, translate(err)
}

func (s *Trips) ListJournalPhotosForTrip(ctx context.Context, tripID uuid.UUID) ([]JournalPhoto, error) {
	photos, err := s.q.ListJournalPhotosForTrip(ctx, tripID)
	if photos == nil {
		photos = []JournalPhoto{}
	}
	return photos, err
}

func (s *Trips) ListJournalPhotosForEntry(ctx context.Context, entryID uuid.UUID) ([]JournalPhoto, error) {
	photos, err := s.q.ListJournalPhotosForEntry(ctx, entryID)
	if photos == nil {
		photos = []JournalPhoto{}
	}
	return photos, err
}

// JournalPhotoWithTrip resolves a photo plus its trip ID, for role-checked
// serving.
type JournalPhotoWithTrip = sqlcgen.JournalPhotoWithTripRow

func (s *Trips) JournalPhotoWithTrip(ctx context.Context, photoID uuid.UUID) (JournalPhotoWithTrip, error) {
	row, err := s.q.JournalPhotoWithTrip(ctx, photoID)
	return row, translate(err)
}

// DeleteJournalPhoto removes the row and returns it so the caller can delete
// the file from disk.
func (s *Trips) DeleteJournalPhoto(ctx context.Context, tripID, photoID uuid.UUID) (JournalPhoto, error) {
	photo, err := s.q.DeleteJournalPhoto(ctx, sqlcgen.DeleteJournalPhotoParams{TripID: tripID, ID: photoID})
	return photo, translate(err)
}
