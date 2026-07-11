-- name: CreateSession :exec
INSERT INTO sessions (token_hash, user_id, expires_at)
VALUES ($1, $2, $3);

-- name: SessionUserByToken :one
-- Resolves an unexpired session to its user and bumps last_seen_at.
UPDATE sessions SET last_seen_at = now()
FROM users
WHERE sessions.token_hash = $1
  AND sessions.expires_at > now()
  AND users.id = sessions.user_id
RETURNING users.*;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE token_hash = $1;

-- name: DeleteSessionsForUser :exec
DELETE FROM sessions WHERE user_id = $1;

-- name: DeleteExpiredSessions :execrows
DELETE FROM sessions WHERE expires_at <= now();
