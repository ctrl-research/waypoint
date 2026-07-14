-- +goose Up
-- Personal calendar-feed token (#52): calendar apps subscribe to
-- /api/v1/calendar/{token}/waypoint.ics, so the token itself is the
-- credential — random, revocable, one per user.
ALTER TABLE users ADD COLUMN calendar_token text UNIQUE;

-- +goose Down
ALTER TABLE users DROP COLUMN calendar_token;
