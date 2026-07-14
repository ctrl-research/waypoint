-- name: EnsureFinalLayer :one
-- Creates the trip's Final layer on first use (new trips get one lazily).
INSERT INTO itinerary_layers (trip_id, owner_id, name)
VALUES ($1, NULL, 'Final')
ON CONFLICT (trip_id) WHERE owner_id IS NULL
DO UPDATE SET name = itinerary_layers.name
RETURNING *;

-- name: ListLayers :many
SELECT * FROM itinerary_layers WHERE trip_id = $1 ORDER BY (owner_id IS NOT NULL), created_at;

-- name: LayerByID :one
SELECT * FROM itinerary_layers WHERE id = $2 AND trip_id = $1;

-- name: EnsureMemberLayer :one
-- A member's proposal layer, created on first use; name/color stick once set.
INSERT INTO itinerary_layers (trip_id, owner_id, name, color)
VALUES ($1, $2, $3, $4)
ON CONFLICT (trip_id, owner_id) WHERE owner_id IS NOT NULL
DO UPDATE SET name = itinerary_layers.name
RETURNING *;

-- name: UpdateLayer :one
UPDATE itinerary_layers SET name = $3, color = $4
WHERE id = $2 AND trip_id = $1
RETURNING *;

-- name: DeleteProposalLayer :execrows
-- The Final layer (owner_id NULL) is never deletable; items cascade.
DELETE FROM itinerary_layers WHERE id = $2 AND trip_id = $1 AND owner_id IS NOT NULL;
