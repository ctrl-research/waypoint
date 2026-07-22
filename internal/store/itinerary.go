package store

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/ctrl-research/waypoint/internal/store/sqlcgen"
)

type (
	ItineraryItem     = sqlcgen.ItineraryItem
	ItineraryCategory = sqlcgen.ItineraryCategory
)

const (
	CategoryActivity  = sqlcgen.ItineraryCategoryActivity
	CategoryFood      = sqlcgen.ItineraryCategoryFood
	CategoryLodging   = sqlcgen.ItineraryCategoryLodging
	CategoryTransport = sqlcgen.ItineraryCategoryTransport
	CategoryFlight    = sqlcgen.ItineraryCategoryFlight
	CategoryTrain     = sqlcgen.ItineraryCategoryTrain
	CategoryOther     = sqlcgen.ItineraryCategoryOther
)

func ValidItineraryCategory(c string) bool {
	switch ItineraryCategory(c) {
	case CategoryActivity, CategoryFood, CategoryLodging, CategoryTransport, CategoryFlight, CategoryTrain, CategoryOther:
		return true
	}
	return false
}

type ItineraryItemParams struct {
	StopID            *uuid.UUID
	DestinationStopID *uuid.UUID // arrival stop for flight/train legs
	OriginHomeID      *uuid.UUID // leg departs from one of the user's homes
	DestinationHomeID *uuid.UUID // leg arrives at one of the user's homes
	Day               time.Time
	StartTime         string // "HH:MM"; "" means unscheduled
	EndTime           string // "HH:MM"; "" means open-ended
	Title             string
	Category          ItineraryCategory
	Notes             string
	CostCents         *int64
	Currency          *string
	Address           string   // venue display address (optional)
	Lat               *float64 // venue coordinates (pair with Lon)
	Lon               *float64
	LayerID           uuid.UUID // which layer the item lives on (#73)
	// Arrival venue for transportation legs — the existing venue is the
	// departure side (#62 follow-up).
	DestinationAddress string
	DestinationLat     *float64
	DestinationLon     *float64
	// Timezone is an IANA timezone name (e.g. "America/Vancouver") for ICS
	// export. Empty means use the server's global fallback or floating time.
	Timezone string
	// ConfirmationCode is a booking reference, PNR, or reservation code.
	ConfirmationCode string
}

// CreateItem appends the item at the end of its day's ordering.
func (s *Trips) CreateItem(ctx context.Context, tripID uuid.UUID, p ItineraryItemParams) (ItineraryItem, error) {
	var tzPtr *string
	if p.Timezone != "" {
		tzPtr = &p.Timezone
	}
	var ccPtr *string
	if p.ConfirmationCode != "" {
		ccPtr = &p.ConfirmationCode
	}
	row, err := s.q.CreateItem(ctx, sqlcgen.CreateItemParams{
		TripID: tripID, StopID: p.StopID, DestinationStopID: p.DestinationStopID,
		OriginHomeID: p.OriginHomeID, DestinationHomeID: p.DestinationHomeID,
		Day: p.Day, StartTime: p.StartTime, EndTime: p.EndTime,
		Title: p.Title, Category: p.Category, Notes: p.Notes,
		CostCents: p.CostCents, Currency: p.Currency,
		Address: p.Address, Lat: p.Lat, Lon: p.Lon, LayerID: p.LayerID,
		DestinationAddress: p.DestinationAddress, DestinationLat: p.DestinationLat, DestinationLon: p.DestinationLon,
		Timezone: tzPtr, ConfirmationCode: ccPtr,
	})
	if err == nil {
		s.touch(ctx, tripID)
	}
	return ItineraryItem(row), translate(err)
}

func (s *Trips) ListItems(ctx context.Context, tripID uuid.UUID) ([]ItineraryItem, error) {
	rows, err := s.q.ListItems(ctx, tripID)
	items := make([]ItineraryItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, ItineraryItem(row))
	}
	return items, err
}

func (s *Trips) ItemByID(ctx context.Context, tripID, itemID uuid.UUID) (ItineraryItem, error) {
	row, err := s.q.ItemByID(ctx, sqlcgen.ItemByIDParams{TripID: tripID, ID: itemID})
	return ItineraryItem(row), translate(err)
}

func (s *Trips) UpdateItem(ctx context.Context, tripID, itemID uuid.UUID, p ItineraryItemParams) (ItineraryItem, error) {
	var tzPtr *string
	if p.Timezone != "" {
		tzPtr = &p.Timezone
	}
	var ccPtr *string
	if p.ConfirmationCode != "" {
		ccPtr = &p.ConfirmationCode
	}
	row, err := s.q.UpdateItem(ctx, sqlcgen.UpdateItemParams{
		TripID: tripID, ID: itemID, StopID: p.StopID, DestinationStopID: p.DestinationStopID,
		OriginHomeID: p.OriginHomeID, DestinationHomeID: p.DestinationHomeID,
		Day: p.Day, StartTime: p.StartTime, EndTime: p.EndTime,
		Title: p.Title, Category: p.Category, Notes: p.Notes,
		CostCents: p.CostCents, Currency: p.Currency,
		Address: p.Address, Lat: p.Lat, Lon: p.Lon, LayerID: p.LayerID,
		DestinationAddress: p.DestinationAddress, DestinationLat: p.DestinationLat, DestinationLon: p.DestinationLon,
		Timezone: tzPtr, ConfirmationCode: ccPtr,
	})
	if err == nil {
		s.touch(ctx, tripID)
	}
	return ItineraryItem(row), translate(err)
}

// ListVisibleItems returns the itinerary — the merge of visible layers.
// Shares, exports, and read-only views use this; the editor sees everything.
func (s *Trips) ListVisibleItems(ctx context.Context, tripID uuid.UUID) ([]ItineraryItem, error) {
	rows, err := s.q.ListVisibleItems(ctx, tripID)
	items := make([]ItineraryItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, ItineraryItem(row))
	}
	return items, err
}

func (s *Trips) DeleteItem(ctx context.Context, tripID, itemID uuid.UUID) error {
	n, err := s.q.DeleteItem(ctx, sqlcgen.DeleteItemParams{TripID: tripID, ID: itemID})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	s.touch(ctx, tripID)
	return nil
}

// ReorderItems sets the ordering of one day's items on one layer to exactly
// ids. It fails with ErrNotFound unless ids is a permutation of that day's
// current items on the layer.
func (s *Trips) ReorderItems(ctx context.Context, tripID uuid.UUID, day time.Time, layerID uuid.UUID, ids []uuid.UUID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := s.q.WithTx(tx)

	count, err := q.CountItemsForDay(ctx, sqlcgen.CountItemsForDayParams{TripID: tripID, Day: day, LayerID: layerID})
	if err != nil {
		return err
	}
	if count != int64(len(ids)) {
		return ErrNotFound
	}

	if err := q.OffsetItemPositions(ctx, sqlcgen.OffsetItemPositionsParams{
		TripID: tripID, Day: day, LayerID: layerID, Position: int32(len(ids)),
	}); err != nil {
		return err
	}
	for i, id := range ids {
		n, err := q.SetItemPosition(ctx, sqlcgen.SetItemPositionParams{
			TripID: tripID, Day: day, LayerID: layerID, ID: id, Position: int32(i),
		})
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrNotFound
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	s.touch(ctx, tripID)
	return nil
}
