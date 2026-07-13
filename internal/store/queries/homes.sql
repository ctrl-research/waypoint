-- name: CreateHome :one
INSERT INTO homes (user_id, name, lat, lon)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListHomes :many
SELECT * FROM homes WHERE user_id = $1 ORDER BY created_at;

-- name: HomeByID :one
SELECT * FROM homes WHERE id = $2 AND user_id = $1;

-- name: DeleteHome :execrows
DELETE FROM homes WHERE id = $2 AND user_id = $1;

-- name: ListTripHomes :many
-- Homes referenced by a trip's itinerary legs (possibly other members'),
-- for labeling "(home) Name" in the UI.
SELECT DISTINCT h.id, h.name
FROM homes h
JOIN itinerary_items i ON h.id IN (i.origin_home_id, i.destination_home_id)
WHERE i.trip_id = $1;
