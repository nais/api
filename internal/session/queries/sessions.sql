-- name: Create :one
INSERT INTO
	sessions (user_id, expires)
VALUES
	(@user_id, @expires)
RETURNING
	*
;

-- name: Get :one
SELECT
	id,
	user_id,
	expires,
	created_at
FROM
	sessions
WHERE
	id = $1
;

-- name: Delete :exec
DELETE FROM sessions
WHERE
	id = $1
;

-- name: SetExpires :one
UPDATE sessions
SET
	expires = $1
WHERE
	id = $2
RETURNING
	*
;
