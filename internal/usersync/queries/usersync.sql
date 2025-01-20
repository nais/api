-- name: List :many
SELECT
	id,
	email,
	name,
	external_id
FROM
	users
ORDER BY
	name,
	email ASC
;

-- name: ListRoles :many
SELECT
	id,
	role_name,
	user_id,
	target_team_slug,
	target_service_account_id
FROM
	user_roles
ORDER BY
	role_name ASC
;

-- name: Create :one
INSERT INTO
	users (name, email, external_id)
VALUES
	($1, LOWER(sqlc.arg('email')), $2)
RETURNING
	id,
	email,
	name,
	external_id
;

-- name: Update :exec
UPDATE users
SET
	name = $1,
	email = LOWER(sqlc.arg('email')),
	external_id = $2
WHERE
	id = $3
;

-- name: Delete :exec
DELETE FROM users
WHERE
	id = $1
;

-- name: AssignGlobalRole :exec
INSERT INTO
	user_roles (user_id, role_name)
VALUES
	($1, $2)
ON CONFLICT DO NOTHING
;

-- name: RevokeGlobalRole :exec
DELETE FROM user_roles
WHERE
	user_id = $1
	AND target_team_slug IS NULL
	AND target_service_account_id IS NULL
	AND role_name = $2
;
