-- name: EnsureMainLayer :one
-- The trip's default layer (owner_id NULL), created on first use.
INSERT INTO itinerary_layers (trip_id, owner_id, name)
VALUES ($1, NULL, 'Main')
ON CONFLICT (trip_id) WHERE owner_id IS NULL
DO UPDATE SET name = itinerary_layers.name
RETURNING *;

-- name: ListLayers :many
SELECT * FROM itinerary_layers WHERE trip_id = $1 ORDER BY (owner_id IS NOT NULL), created_at;

-- name: LayerByID :one
SELECT * FROM itinerary_layers WHERE id = $2 AND trip_id = $1;

-- name: CreateLayer :one
-- A member's named layer; anyone on the trip can hold several.
INSERT INTO itinerary_layers (trip_id, owner_id, name, color)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: UpdateLayer :one
UPDATE itinerary_layers SET name = $3, color = $4, visible = $5
WHERE id = $2 AND trip_id = $1
RETURNING *;

-- name: DeleteProposalLayer :execrows
-- The Main layer (owner_id NULL) is never deletable; items cascade.
DELETE FROM itinerary_layers WHERE id = $2 AND trip_id = $1 AND owner_id IS NOT NULL;
