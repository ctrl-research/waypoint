-- +goose Up
-- Hotels, ferries, flights, and trains carry confirmation codes or PNRs.
-- Storing them in notes loses structure. This column is nullable; empty
-- string is fine too — the application layer treats both as unset.
ALTER TABLE itinerary_items ADD COLUMN confirmation_code text;

-- +goose Down
ALTER TABLE itinerary_items DROP COLUMN confirmation_code;
