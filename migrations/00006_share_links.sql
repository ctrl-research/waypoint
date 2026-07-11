-- +goose Up
CREATE TABLE share_links (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    trip_id    uuid NOT NULL REFERENCES trips (id) ON DELETE CASCADE,
    -- Unguessable URL token (256-bit, base64url). Stored plaintext so the
    -- owner can re-copy the link; it grants read-only access to one trip
    -- and is individually revocable.
    token      text NOT NULL UNIQUE,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX share_links_trip_idx ON share_links (trip_id);

-- +goose Down
DROP TABLE share_links;
