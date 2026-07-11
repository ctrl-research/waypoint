package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type ItineraryCategory string

const (
	CategoryActivity  ItineraryCategory = "activity"
	CategoryFood      ItineraryCategory = "food"
	CategoryLodging   ItineraryCategory = "lodging"
	CategoryTransport ItineraryCategory = "transport"
	CategoryOther     ItineraryCategory = "other"
)

func ValidItineraryCategory(c string) bool {
	switch ItineraryCategory(c) {
	case CategoryActivity, CategoryFood, CategoryLodging, CategoryTransport, CategoryOther:
		return true
	}
	return false
}

type ItineraryItem struct {
	ID        uuid.UUID
	TripID    uuid.UUID
	StopID    *uuid.UUID
	Day       time.Time
	StartTime *string // "HH:MM", nil when unscheduled
	Title     string
	Category  ItineraryCategory
	Notes     string
	CostCents *int64
	Currency  *string
	Position  int32
}

type ItineraryItemParams struct {
	StopID    *uuid.UUID
	Day       time.Time
	StartTime *string // "HH:MM"
	Title     string
	Category  ItineraryCategory
	Notes     string
	CostCents *int64
	Currency  *string
}

const itemColumns = `id, trip_id, stop_id, day, to_char(start_time, 'HH24:MI'), title, category, notes, cost_cents, currency, position`

func scanItem(row pgx.Row) (ItineraryItem, error) {
	var it ItineraryItem
	err := row.Scan(&it.ID, &it.TripID, &it.StopID, &it.Day, &it.StartTime,
		&it.Title, &it.Category, &it.Notes, &it.CostCents, &it.Currency, &it.Position)
	if err == pgx.ErrNoRows {
		return ItineraryItem{}, ErrNotFound
	}
	return it, err
}

// CreateItem appends the item at the end of its day's ordering.
func (s *Trips) CreateItem(ctx context.Context, tripID uuid.UUID, p ItineraryItemParams) (ItineraryItem, error) {
	it, err := scanItem(s.pool.QueryRow(ctx, `
		INSERT INTO itinerary_items (trip_id, stop_id, day, start_time, title, category, notes, cost_cents, currency, position)
		VALUES ($1, $2, $3, $4::time, $5, $6, $7, $8, $9,
		        (SELECT COALESCE(MAX(position) + 1, 0) FROM itinerary_items WHERE trip_id = $1 AND day = $3))
		RETURNING `+itemColumns,
		tripID, p.StopID, p.Day, p.StartTime, p.Title, p.Category, p.Notes, p.CostCents, p.Currency))
	if err == nil {
		s.touch(ctx, tripID)
	}
	return it, err
}

func (s *Trips) ListItems(ctx context.Context, tripID uuid.UUID) ([]ItineraryItem, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT `+itemColumns+` FROM itinerary_items WHERE trip_id = $1 ORDER BY day, position`, tripID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []ItineraryItem{}
	for rows.Next() {
		it, err := scanItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

func (s *Trips) ItemByID(ctx context.Context, tripID, itemID uuid.UUID) (ItineraryItem, error) {
	return scanItem(s.pool.QueryRow(ctx,
		`SELECT `+itemColumns+` FROM itinerary_items WHERE id = $2 AND trip_id = $1`, tripID, itemID))
}

func (s *Trips) UpdateItem(ctx context.Context, tripID, itemID uuid.UUID, p ItineraryItemParams) (ItineraryItem, error) {
	it, err := scanItem(s.pool.QueryRow(ctx, `
		UPDATE itinerary_items
		SET stop_id = $3, day = $4, start_time = $5::time, title = $6, category = $7,
		    notes = $8, cost_cents = $9, currency = $10
		WHERE id = $2 AND trip_id = $1
		RETURNING `+itemColumns,
		tripID, itemID, p.StopID, p.Day, p.StartTime, p.Title, p.Category, p.Notes, p.CostCents, p.Currency))
	if err == nil {
		s.touch(ctx, tripID)
	}
	return it, err
}

func (s *Trips) DeleteItem(ctx context.Context, tripID, itemID uuid.UUID) error {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM itinerary_items WHERE id = $2 AND trip_id = $1`, tripID, itemID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	s.touch(ctx, tripID)
	return nil
}
