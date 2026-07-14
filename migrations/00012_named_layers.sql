-- +goose Up
-- Layers become free-form (#73 follow-up): members create as many named
-- layers as they like, then compile items into the shared "Plan" layer
-- (owner_id NULL, formerly "Final").
DROP INDEX itinerary_layers_owner_uq;
UPDATE itinerary_layers SET name = 'Plan' WHERE owner_id IS NULL;

-- +goose Down
UPDATE itinerary_layers SET name = 'Final' WHERE owner_id IS NULL;
CREATE UNIQUE INDEX itinerary_layers_owner_uq
    ON itinerary_layers (trip_id, owner_id) WHERE owner_id IS NOT NULL;
