-- +goose Up
-- Per-item timezone (IANA name) for ICS export. When set, item times are
-- converted to UTC and emitted with a Z suffix so Google Calendar imports
-- them correctly. Falls back to WAYPOINT_TIMEZONE, then floating time.
ALTER TABLE itinerary_items ADD COLUMN timezone text;

-- +goose Down
ALTER TABLE itinerary_items DROP COLUMN timezone;
