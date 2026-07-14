-- +goose Up
-- Layers for the collaborative itinerary editor (#73). owner_id NULL marks
-- the single Final layer per trip — the one the trip page displays; every
-- member can hold their own proposal layer (slice 2).
CREATE TABLE itinerary_layers (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    trip_id    uuid NOT NULL REFERENCES trips (id) ON DELETE CASCADE,
    owner_id   uuid REFERENCES users (id) ON DELETE CASCADE,
    name       text NOT NULL,
    color      text NOT NULL DEFAULT '#2a78d6',
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX itinerary_layers_final_uq ON itinerary_layers (trip_id) WHERE owner_id IS NULL;
CREATE INDEX itinerary_layers_trip_idx ON itinerary_layers (trip_id);

-- Every existing item moves onto its trip's Final layer.
ALTER TABLE itinerary_items
    ADD COLUMN layer_id uuid REFERENCES itinerary_layers (id) ON DELETE CASCADE;

INSERT INTO itinerary_layers (trip_id, owner_id, name)
SELECT DISTINCT trip_id, NULL::uuid, 'Final' FROM itinerary_items;

UPDATE itinerary_items i
SET layer_id = l.id
FROM itinerary_layers l
WHERE l.trip_id = i.trip_id AND l.owner_id IS NULL;

ALTER TABLE itinerary_items ALTER COLUMN layer_id SET NOT NULL;

-- +goose Down
ALTER TABLE itinerary_items DROP COLUMN layer_id;
DROP TABLE itinerary_layers;
