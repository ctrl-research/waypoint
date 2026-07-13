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
