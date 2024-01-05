-- name: CreateSession :one
INSERT INTO sessions (user_id, expires)
VALUES ($1, $2)
RETURNING *;

-- name: GetSessionByID :one
SELECT * FROM sessions
WHERE id = $1;

-- name: DeleteSession :exec
DELETE FROM sessions
WHERE id = $1;

-- name: SetSessionExpires :one
UPDATE sessions
SET expires = $1
WHERE id = $2
RETURNING *;