-- name: CreateShareLink :one
INSERT INTO share_links (trip_id, token)
VALUES ($1, $2)
RETURNING *;

-- name: ListShareLinks :many
SELECT * FROM share_links WHERE trip_id = $1 ORDER BY created_at;

-- name: DeleteShareLink :execrows
DELETE FROM share_links WHERE trip_id = $1 AND id = $2;

-- name: TripByShareToken :one
SELECT t.* FROM trips t
JOIN share_links s ON s.trip_id = t.id
WHERE s.token = $1;
