-- name: CreateUser :one
INSERT INTO users (email, display_name, avatar_url, google_sub, password_hash, is_admin)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: UserByID :one
SELECT * FROM users WHERE id = $1;

-- name: UserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: UserByGoogleSub :one
SELECT * FROM users WHERE google_sub = $1;

-- name: UpdateUserProfile :one
UPDATE users SET display_name = $2, avatar_url = $3 WHERE id = $1
RETURNING *;

-- name: LinkGoogle :one
UPDATE users SET google_sub = $2, display_name = $3, avatar_url = $4
WHERE id = $1
RETURNING *;

-- name: SetPassword :one
UPDATE users SET password_hash = $2 WHERE id = $1
RETURNING *;

-- name: CountUsers :one
SELECT count(*) FROM users;
