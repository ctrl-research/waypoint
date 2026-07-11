-- +goose Up
CREATE TYPE trip_status AS ENUM ('planning', 'active', 'completed');

CREATE TABLE trips (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    title       text NOT NULL,
    description text NOT NULL DEFAULT '',
    status      trip_status NOT NULL DEFAULT 'planning',
    start_date  date,
    end_date    date,
    cover_photo text,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX trips_owner_id_idx ON trips (owner_id);

CREATE TABLE stops (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    trip_id        uuid NOT NULL REFERENCES trips (id) ON DELETE CASCADE,
    name           text NOT NULL,
    lat            double precision,
    lon            double precision,
    arrival_date   date,
    departure_date date,
    position       integer NOT NULL,
    notes          text NOT NULL DEFAULT '',

    -- a stop has either both coordinates or none
    CONSTRAINT stops_latlon_pair CHECK ((lat IS NULL) = (lon IS NULL))
);

CREATE INDEX stops_trip_position_idx ON stops (trip_id, position);

CREATE TYPE itinerary_category AS ENUM ('activity', 'food', 'lodging', 'transport', 'other');

CREATE TABLE itinerary_items (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    trip_id    uuid NOT NULL REFERENCES trips (id) ON DELETE CASCADE,
    stop_id    uuid REFERENCES stops (id) ON DELETE SET NULL,
    day        date NOT NULL,
    start_time time,
    title      text NOT NULL,
    category   itinerary_category NOT NULL DEFAULT 'other',
    notes      text NOT NULL DEFAULT '',
    cost_cents bigint,
    currency   char(3),
    position   integer NOT NULL,

    CONSTRAINT items_cost_currency_pair CHECK ((cost_cents IS NULL) = (currency IS NULL))
);

CREATE INDEX itinerary_items_trip_day_position_idx ON itinerary_items (trip_id, day, position);

-- +goose Down
DROP TABLE itinerary_items;
DROP TYPE itinerary_category;
DROP TABLE stops;
DROP TABLE trips;
DROP TYPE trip_status;
