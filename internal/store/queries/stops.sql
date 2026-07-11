-- name: CreateStop :one
-- Appends the stop at the end of the trip's ordering.
INSERT INTO stops (trip_id, name, lat, lon, arrival_date, departure_date, notes, position)
VALUES ($1, $2, $3, $4, $5, $6, $7,
        (SELECT COALESCE(MAX(position) + 1, 0) FROM stops WHERE trip_id = $1))
RETURNING *;

-- name: ListStops :many
SELECT * FROM stops WHERE trip_id = $1 ORDER BY position;

-- name: StopByID :one
SELECT * FROM stops WHERE id = $2 AND trip_id = $1;

-- name: UpdateStop :one
UPDATE stops
SET name = $3, lat = $4, lon = $5, arrival_date = $6, departure_date = $7, notes = $8
WHERE id = $2 AND trip_id = $1
RETURNING *;

-- name: DeleteStop :execrows
DELETE FROM stops WHERE id = $2 AND trip_id = $1;

-- name: CountStops :one
SELECT count(*) FROM stops WHERE trip_id = $1;

-- name: OffsetStopPositions :exec
-- Shifts all positions up so a reorder never collides with target positions.
UPDATE stops SET position = position + $2 WHERE trip_id = $1;

-- name: SetStopPosition :execrows
UPDATE stops SET position = $3 WHERE id = $2 AND trip_id = $1;
