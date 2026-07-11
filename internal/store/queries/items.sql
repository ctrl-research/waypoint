-- start_time is a `time` column exposed as an "HH:MM" string; '' means
-- unscheduled. NULLIF on the way in and COALESCE(to_char(...)) on the way
-- out keep sqlc's generated types plain strings.

-- name: CreateItem :one
-- Appends the item at the end of its day's ordering.
INSERT INTO itinerary_items (trip_id, stop_id, day, start_time, title, category, notes, cost_cents, currency, position)
VALUES (sqlc.arg(trip_id), sqlc.arg(stop_id), sqlc.arg(day), NULLIF(sqlc.arg(start_time)::text, '')::time,
        sqlc.arg(title), sqlc.arg(category), sqlc.arg(notes), sqlc.arg(cost_cents), sqlc.arg(currency),
        (SELECT COALESCE(MAX(position) + 1, 0) FROM itinerary_items i WHERE i.trip_id = sqlc.arg(trip_id) AND i.day = sqlc.arg(day)))
RETURNING id, trip_id, stop_id, day, CAST(COALESCE(to_char(start_time, 'HH24:MI'), '') AS text) AS start_time, title, category, notes, cost_cents, currency, position;

-- name: ListItems :many
SELECT id, trip_id, stop_id, day, CAST(COALESCE(to_char(start_time, 'HH24:MI'), '') AS text) AS start_time, title, category, notes, cost_cents, currency, position
FROM itinerary_items WHERE trip_id = $1 ORDER BY day, position;

-- name: ItemByID :one
SELECT id, trip_id, stop_id, day, CAST(COALESCE(to_char(start_time, 'HH24:MI'), '') AS text) AS start_time, title, category, notes, cost_cents, currency, position
FROM itinerary_items WHERE id = $2 AND trip_id = $1;

-- name: UpdateItem :one
UPDATE itinerary_items
SET stop_id = sqlc.arg(stop_id), day = sqlc.arg(day), start_time = NULLIF(sqlc.arg(start_time)::text, '')::time,
    title = sqlc.arg(title), category = sqlc.arg(category), notes = sqlc.arg(notes),
    cost_cents = sqlc.arg(cost_cents), currency = sqlc.arg(currency)
WHERE id = sqlc.arg(id) AND trip_id = sqlc.arg(trip_id)
RETURNING id, trip_id, stop_id, day, CAST(COALESCE(to_char(start_time, 'HH24:MI'), '') AS text) AS start_time, title, category, notes, cost_cents, currency, position;

-- name: DeleteItem :execrows
DELETE FROM itinerary_items WHERE id = $2 AND trip_id = $1;

-- name: CountItemsForDay :one
SELECT count(*) FROM itinerary_items WHERE trip_id = $1 AND day = $2;

-- name: OffsetItemPositions :exec
UPDATE itinerary_items SET position = position + $3 WHERE trip_id = $1 AND day = $2;

-- name: SetItemPosition :execrows
UPDATE itinerary_items SET position = $4 WHERE id = $3 AND trip_id = $1 AND day = $2;
