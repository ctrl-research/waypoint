-- name: ListLocatedStopsForUser :many
-- Every located stop across trips the user can see, in route order —
-- the raw material for the stats page and visited-places map.
SELECT s.name, s.lat, s.lon, s.position, t.id AS trip_id, t.title AS trip_title
FROM stops s
JOIN trips t ON t.id = s.trip_id
LEFT JOIN trip_members m ON m.trip_id = t.id AND m.user_id = $1
WHERE (t.owner_id = $1 OR m.user_id IS NOT NULL)
  AND s.lat IS NOT NULL
ORDER BY t.id, s.position;

-- name: ListFlightLegsForUser :many
-- Flight items across accessible trips, with the departure/arrival stop
-- coordinates when both stops are located.
SELECT CAST(COALESCE(to_char(i.start_time, 'HH24:MI'), '') AS text) AS start_time,
       CAST(COALESCE(to_char(i.end_time, 'HH24:MI'), '') AS text) AS end_time,
       s1.lat AS from_lat, s1.lon AS from_lon,
       s2.lat AS to_lat, s2.lon AS to_lon
FROM itinerary_items i
JOIN trips t ON t.id = i.trip_id
LEFT JOIN trip_members m ON m.trip_id = t.id AND m.user_id = $1
LEFT JOIN stops s1 ON s1.id = i.stop_id
LEFT JOIN stops s2 ON s2.id = i.destination_stop_id
WHERE i.category = 'flight' AND (t.owner_id = $1 OR m.user_id IS NOT NULL);

-- name: CountDistinctStopNamesForUser :one
-- All distinct stop names (located or not) across accessible trips — the
-- denominator for the Cities stat tile.
SELECT count(DISTINCT lower(trim(s.name)))
FROM stops s
JOIN trips t ON t.id = s.trip_id
LEFT JOIN trip_members m ON m.trip_id = t.id AND m.user_id = $1
WHERE t.owner_id = $1 OR m.user_id IS NOT NULL;
