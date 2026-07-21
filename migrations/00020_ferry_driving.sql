-- +goose Up
-- Ferries (#114) and driving/road-trips (#113) are distinct from generic
-- 'transport'. Ferries have vessel/booking/check-in semantics like flights and
-- trains; long drives need leg treatment so the replay and timeline don't treat
-- them as local hops like a taxi.
ALTER TYPE itinerary_category ADD VALUE IF NOT EXISTS 'ferry';
ALTER TYPE itinerary_category ADD VALUE IF NOT EXISTS 'driving';

-- +goose Down
-- Postgres cannot drop enum values, so the Down migration is a no-op. Reversing
-- this migration would require recreating the type, which is not safe while data
-- exists. See the related issue for a discussion on the migration path back.
