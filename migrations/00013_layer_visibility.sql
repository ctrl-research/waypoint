-- +goose Up
-- The itinerary becomes the merge of all visible layers (#73 follow-up):
-- visibility is shared trip state, not a per-session view filter. The
-- promote-to-Plan concept goes away, so the default layer is just "Main".
ALTER TABLE itinerary_layers ADD COLUMN visible boolean NOT NULL DEFAULT true;
UPDATE itinerary_layers SET name = 'Main' WHERE owner_id IS NULL;

-- +goose Down
ALTER TABLE itinerary_layers DROP COLUMN visible;
UPDATE itinerary_layers SET name = 'Plan' WHERE owner_id IS NULL;
