-- name: CreateSession :one
INSERT INTO sessions (user_id, expires)
VALUES (@user_id, @expires)
RETURNING *;

-- name: GetSessionByID :one
SELECT * FROM sessions
WHERE id = @id;

-- name: DeleteSession :exec
DELETE FROM sessions
WHERE id = @id;

-- name: SetSessionExpires :one
UPDATE sessions
SET expires = @expires
WHERE id = @id
RETURNING *;
