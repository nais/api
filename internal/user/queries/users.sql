-- name: List :many
SELECT
	sqlc.embed(users),
	COUNT(*) OVER () AS total_count
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
	id = ANY (@ids::UUID[])
ORDER BY
	name,
	email ASC
;

-- name: GetByEmail :one
SELECT
	*
FROM
	users
WHERE
	email = LOWER(@email)
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

-- name: GetByExternalID :one
SELECT
	*
FROM
	users
WHERE
	external_id = @external_id
;
