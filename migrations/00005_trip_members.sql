-- +goose Up
CREATE TYPE trip_role AS ENUM ('viewer', 'editor');

CREATE TABLE trip_members (
    trip_id    uuid NOT NULL REFERENCES trips (id) ON DELETE CASCADE,
    user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role       trip_role NOT NULL DEFAULT 'viewer',
    created_at timestamptz NOT NULL DEFAULT now(),

    PRIMARY KEY (trip_id, user_id)
);

CREATE INDEX trip_members_user_idx ON trip_members (user_id);

-- +goose Down
DROP TABLE trip_members;
DROP TYPE trip_role;
