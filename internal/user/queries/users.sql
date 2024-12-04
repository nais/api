-- name: Create :one
INSERT INTO
	users (name, email, external_id)
VALUES
	(@name, LOWER(@email), @external_id)
RETURNING
	*
;

-- name: Count :one
SELECT
	COUNT(*)
FROM
	users
;

-- name: List :many
SELECT
	*
FROM
	users
ORDER BY
	CASE
		WHEN @order_by::TEXT = 'name:asc' THEN name
	END ASC,
	CASE
		WHEN @order_by::TEXT = 'name:desc' THEN name
	END DESC,
	CASE
		WHEN @order_by::TEXT = 'email:asc' THEN email
	END ASC,
	CASE
		WHEN @order_by::TEXT = 'email:desc' THEN email
	END DESC,
	name,
	email ASC
LIMIT
	sqlc.arg('limit')
OFFSET
	sqlc.arg('offset')
;

-- name: GetByIDs :many
SELECT
	*
FROM
	users
WHERE
	id = ANY (@ids::UUID [])
ORDER BY
	name,
	email ASC
;

-- name: Get :one
SELECT
	*
FROM
	users
WHERE
	id = @id
;

-- name: GetByExternalID :one
SELECT
	*
FROM
	users
WHERE
	external_id = @external_id
;

-- name: GetByEmail :one
SELECT
	*
FROM
	users
WHERE
	email = LOWER(@email)
;

-- name: ListMemberships :many
SELECT
	sqlc.embed(teams),
	user_roles.role_name
FROM
	user_roles
	JOIN teams ON teams.slug = user_roles.target_team_slug
WHERE
	user_roles.user_id = @user_id
ORDER BY
	teams.slug ASC
LIMIT
	sqlc.arg('limit')
OFFSET
	sqlc.arg('offset')
;

-- name: CountMemberships :one
SELECT
	COUNT(*)
FROM
	user_roles
WHERE
	user_roles.user_id = @user_id
	AND target_team_slug IS NOT NULL
;

-- name: Update :one
UPDATE users
SET
	name = @name,
	email = LOWER(@email),
	external_id = @external_id
WHERE
	id = @id
RETURNING
	*
;

-- name: Delete :exec
DELETE FROM users
WHERE
	id = @id
;

-- name: ListGCPGroupsForUser :one
SELECT
	ARRAY_AGG(teams.google_group_email)::TEXT[]
FROM
	teams
	JOIN user_roles ON user_roles.target_team_slug = teams.slug
WHERE
	user_roles.user_id = @user_id
	AND teams.google_group_email IS NOT NULL
;
