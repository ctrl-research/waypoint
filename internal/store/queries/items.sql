-- start_time/end_time are `time` columns exposed as "HH:MM" strings; ''
-- means unset. NULLIF on the way in and COALESCE(to_char(...)) on the way
-- out keep sqlc's generated types plain strings.

-- name: CreateItem :one
-- Appends the item at the end of its day's ordering.
INSERT INTO itinerary_items (trip_id, stop_id, destination_stop_id, origin_home_id, destination_home_id, day, start_time, end_time, title, category, notes, cost_cents, currency, address, lat, lon, layer_id, position)
VALUES (sqlc.arg(trip_id), sqlc.arg(stop_id), sqlc.arg(destination_stop_id), sqlc.arg(origin_home_id), sqlc.arg(destination_home_id), sqlc.arg(day),
        NULLIF(sqlc.arg(start_time)::text, '')::time, NULLIF(sqlc.arg(end_time)::text, '')::time,
        sqlc.arg(title), sqlc.arg(category), sqlc.arg(notes), sqlc.arg(cost_cents), sqlc.arg(currency),
        sqlc.arg(address), sqlc.arg(lat), sqlc.arg(lon), sqlc.arg(layer_id),
        (SELECT COALESCE(MAX(position) + 1, 0) FROM itinerary_items i WHERE i.trip_id = sqlc.arg(trip_id) AND i.day = sqlc.arg(day)))
RETURNING id, trip_id, stop_id, day,
          CAST(COALESCE(to_char(start_time, 'HH24:MI'), '') AS text) AS start_time,
          title, category, notes, cost_cents, currency, position,
          CAST(COALESCE(to_char(end_time, 'HH24:MI'), '') AS text) AS end_time,
          destination_stop_id, origin_home_id, destination_home_id, address, lat, lon, layer_id;

-- name: ListItems :many
SELECT id, trip_id, stop_id, day,
       CAST(COALESCE(to_char(start_time, 'HH24:MI'), '') AS text) AS start_time,
       title, category, notes, cost_cents, currency, position,
       CAST(COALESCE(to_char(end_time, 'HH24:MI'), '') AS text) AS end_time,
       destination_stop_id, origin_home_id, destination_home_id, address, lat, lon, layer_id
FROM itinerary_items WHERE trip_id = $1 ORDER BY day, position, id;

-- name: ListVisibleItems :many
-- The itinerary is the merge of visible layers — what shares, exports,
-- and read-only views see.
SELECT i.id, i.trip_id, i.stop_id, i.day,
       CAST(COALESCE(to_char(i.start_time, 'HH24:MI'), '') AS text) AS start_time,
       i.title, i.category, i.notes, i.cost_cents, i.currency, i.position,
       CAST(COALESCE(to_char(i.end_time, 'HH24:MI'), '') AS text) AS end_time,
       i.destination_stop_id, i.origin_home_id, i.destination_home_id, i.address, i.lat, i.lon, i.layer_id
FROM itinerary_items i
JOIN itinerary_layers l ON l.id = i.layer_id AND l.visible
WHERE i.trip_id = $1 ORDER BY i.day, i.position, i.id;

-- name: ItemByID :one
SELECT id, trip_id, stop_id, day,
       CAST(COALESCE(to_char(start_time, 'HH24:MI'), '') AS text) AS start_time,
       title, category, notes, cost_cents, currency, position,
       CAST(COALESCE(to_char(end_time, 'HH24:MI'), '') AS text) AS end_time,
       destination_stop_id, origin_home_id, destination_home_id, address, lat, lon, layer_id
FROM itinerary_items WHERE id = $2 AND trip_id = $1;

-- name: UpdateItem :one
UPDATE itinerary_items
SET stop_id = sqlc.arg(stop_id), destination_stop_id = sqlc.arg(destination_stop_id),
    origin_home_id = sqlc.arg(origin_home_id), destination_home_id = sqlc.arg(destination_home_id), day = sqlc.arg(day),
    start_time = NULLIF(sqlc.arg(start_time)::text, '')::time,
    end_time = NULLIF(sqlc.arg(end_time)::text, '')::time,
    title = sqlc.arg(title), category = sqlc.arg(category), notes = sqlc.arg(notes),
    cost_cents = sqlc.arg(cost_cents), currency = sqlc.arg(currency),
    address = sqlc.arg(address), lat = sqlc.arg(lat), lon = sqlc.arg(lon),
    layer_id = sqlc.arg(layer_id)
WHERE id = sqlc.arg(id) AND trip_id = sqlc.arg(trip_id)
RETURNING id, trip_id, stop_id, day,
          CAST(COALESCE(to_char(start_time, 'HH24:MI'), '') AS text) AS start_time,
          title, category, notes, cost_cents, currency, position,
          CAST(COALESCE(to_char(end_time, 'HH24:MI'), '') AS text) AS end_time,
          destination_stop_id, origin_home_id, destination_home_id, address, lat, lon, layer_id;

-- name: DeleteItem :execrows
DELETE FROM itinerary_items WHERE id = $2 AND trip_id = $1;

-- Reordering is scoped to one layer (#73 slice 2): each layer's board
-- orders its own items, so positions may repeat across layers on a day.

-- name: CountItemsForDay :one
SELECT count(*) FROM itinerary_items WHERE trip_id = $1 AND day = $2 AND layer_id = $3;

-- name: OffsetItemPositions :exec
UPDATE itinerary_items SET position = position + $4 WHERE trip_id = $1 AND day = $2 AND layer_id = $3;

-- name: SetItemPosition :execrows
UPDATE itinerary_items SET position = $5 WHERE id = $4 AND trip_id = $1 AND day = $2 AND layer_id = $3;
