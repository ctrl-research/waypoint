-- name: UpsertMember :one
-- Adding an existing member updates their role instead of failing.
INSERT INTO trip_members (trip_id, user_id, role)
VALUES ($1, $2, $3)
ON CONFLICT (trip_id, user_id) DO UPDATE SET role = EXCLUDED.role
RETURNING *;

-- name: ListMembers :many
SELECT m.user_id, m.role, m.created_at, u.email, u.display_name, u.avatar_url
FROM trip_members m
JOIN users u ON u.id = m.user_id
WHERE m.trip_id = $1
ORDER BY m.created_at;

-- name: RemoveMember :execrows
DELETE FROM trip_members WHERE trip_id = $1 AND user_id = $2;

-- name: TripWithRole :one
-- Loads a trip together with the requesting user's role on it: 'owner',
-- a membership role, or '' for no access.
SELECT sqlc.embed(t),
       CAST(CASE WHEN t.owner_id = $2 THEN 'owner' ELSE COALESCE(m.role::text, '') END AS text) AS role
FROM trips t
LEFT JOIN trip_members m ON m.trip_id = t.id AND m.user_id = $2
WHERE t.id = $1;

-- name: ListAccessibleTrips :many
-- Trips the user owns or is a member of, with their role annotated.
SELECT sqlc.embed(t),
       CAST(CASE WHEN t.owner_id = $1 THEN 'owner' ELSE m.role::text END AS text) AS role
FROM trips t
LEFT JOIN trip_members m ON m.trip_id = t.id AND m.user_id = $1
WHERE t.owner_id = $1 OR m.user_id IS NOT NULL
ORDER BY COALESCE(t.start_date, t.created_at::date) DESC, t.created_at DESC;
