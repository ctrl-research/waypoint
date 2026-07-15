-- +goose Up
-- What scale of place an area is (country, city, town… — Nominatim's
-- addresstype), captured at creation so focusing an area without items
-- can pick a sensible zoom (#93). Existing rows keep '' and get a
-- city-ish default.
ALTER TABLE stops ADD COLUMN kind text NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE stops DROP COLUMN kind;
