-- +goose Up
-- Itinerary items can carry their own venue: a display address plus
-- coordinates, so map pins sit at the restaurant/museum rather than under
-- the stop marker.
ALTER TABLE itinerary_items
    ADD COLUMN address text NOT NULL DEFAULT '',
    ADD COLUMN lat double precision,
    ADD COLUMN lon double precision,
    ADD CONSTRAINT items_latlon_pair CHECK ((lat IS NULL) = (lon IS NULL));

-- +goose Down
ALTER TABLE itinerary_items DROP CONSTRAINT items_latlon_pair;
ALTER TABLE itinerary_items DROP COLUMN address, DROP COLUMN lat, DROP COLUMN lon;
