-- name: CreateTrip :one
INSERT INTO trips (owner_id, title, description, status, start_date, end_date, cover_photo)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: TripByID :one
SELECT * FROM trips WHERE id = $1;

-- name: ListTripsByOwner :many
SELECT * FROM trips
WHERE owner_id = $1
ORDER BY COALESCE(start_date, created_at::date) DESC, created_at DESC;

-- name: UpdateTrip :one
UPDATE trips
SET title = $2, description = $3, status = $4, start_date = $5,
    end_date = $6, cover_photo = $7, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteTrip :execrows
DELETE FROM trips WHERE id = $1;

-- name: TouchTrip :exec
UPDATE trips SET updated_at = now() WHERE id = $1;
