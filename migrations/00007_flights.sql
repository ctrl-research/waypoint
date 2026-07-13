-- +goose Up
-- Flights are itinerary items: category 'flight' departs from stop_id and
-- lands at destination_stop_id; end_time lets any item carry a duration.
ALTER TYPE itinerary_category ADD VALUE IF NOT EXISTS 'flight';

ALTER TABLE itinerary_items
    ADD COLUMN end_time time,
    ADD COLUMN destination_stop_id uuid REFERENCES stops (id) ON DELETE SET NULL;

-- +goose Down
-- Postgres cannot drop enum values; only the columns are reversible.
ALTER TABLE itinerary_items
    DROP COLUMN end_time,
    DROP COLUMN destination_stop_id;
