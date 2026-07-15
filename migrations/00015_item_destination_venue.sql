-- +goose Up
-- Transportation items are legs: the existing venue is the departure
-- station/airport and this is the arrival one, so the map and replay can
-- draw the actual journey (#62 follow-up).
ALTER TABLE itinerary_items
    ADD COLUMN destination_address text NOT NULL DEFAULT '',
    ADD COLUMN destination_lat double precision,
    ADD COLUMN destination_lon double precision,
    ADD CONSTRAINT items_dest_latlon_pair CHECK ((destination_lat IS NULL) = (destination_lon IS NULL));

-- +goose Down
ALTER TABLE itinerary_items DROP CONSTRAINT items_dest_latlon_pair;
ALTER TABLE itinerary_items DROP COLUMN destination_address, DROP COLUMN destination_lat, DROP COLUMN destination_lon;
