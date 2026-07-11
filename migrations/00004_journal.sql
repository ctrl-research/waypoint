-- +goose Up
CREATE TABLE journal_entries (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    trip_id    uuid NOT NULL REFERENCES trips (id) ON DELETE CASCADE,
    author_id  uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    entry_date date NOT NULL,
    title      text NOT NULL DEFAULT '',
    body       text NOT NULL DEFAULT '',
    lat        double precision,
    lon        double precision,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT journal_latlon_pair CHECK ((lat IS NULL) = (lon IS NULL))
);

CREATE INDEX journal_entries_trip_date_idx ON journal_entries (trip_id, entry_date);

CREATE TABLE journal_photos (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    entry_id     uuid NOT NULL REFERENCES journal_entries (id) ON DELETE CASCADE,
    file_path    text NOT NULL,
    content_type text NOT NULL,
    size_bytes   bigint NOT NULL,
    taken_at     timestamptz,
    lat          double precision,
    lon          double precision,
    caption      text NOT NULL DEFAULT '',
    created_at   timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT photo_latlon_pair CHECK ((lat IS NULL) = (lon IS NULL))
);

CREATE INDEX journal_photos_entry_idx ON journal_photos (entry_id);

-- +goose Down
DROP TABLE journal_photos;
DROP TABLE journal_entries;
