-- +goose Up
-- One proposal layer per member per trip (#73 slice 2); the Final layer
-- keeps its own partial unique index from 00010.
CREATE UNIQUE INDEX itinerary_layers_owner_uq
    ON itinerary_layers (trip_id, owner_id) WHERE owner_id IS NOT NULL;

-- +goose Down
DROP INDEX itinerary_layers_owner_uq;
