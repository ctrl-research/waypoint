-- +goose Up
-- Trains get the same from/to treatment as flights.
ALTER TYPE itinerary_category ADD VALUE IF NOT EXISTS 'train';

-- A user's home locations: implicit endpoints for flight/train legs that
-- start or end outside any trip stop ("fly home"). Users can have several
-- (e.g. apartment and family home).
CREATE TABLE homes (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name       text NOT NULL,
    lat        double precision NOT NULL,
    lon        double precision NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX homes_user_idx ON homes (user_id);

-- Flight/train legs can start or end at a home instead of a stop.
ALTER TABLE itinerary_items
    ADD COLUMN origin_home_id uuid REFERENCES homes (id) ON DELETE SET NULL,
    ADD COLUMN destination_home_id uuid REFERENCES homes (id) ON DELETE SET NULL;

-- +goose Down
ALTER TABLE itinerary_items DROP COLUMN origin_home_id, DROP COLUMN destination_home_id;
DROP TABLE homes;
