-- name: ListForTeam :many
SELECT
	*
FROM
	activity_log_entries
WHERE
	team_slug = @team_slug
ORDER BY
	created_at DESC
LIMIT
	sqlc.arg('limit')
OFFSET
	sqlc.arg('offset')
;

-- name: ListForResource :many
SELECT
	*
FROM
	activity_log_entries
WHERE
	resource_type = @resource_type
	AND resource_name = @resource_name
ORDER BY
	created_at DESC
LIMIT
	sqlc.arg('limit')
OFFSET
	sqlc.arg('offset')
;

-- name: ListForTeamByResource :many
SELECT
	*
FROM
	activity_log_entries
WHERE
	team_slug = @team_slug
	AND resource_type = @resource_type
ORDER BY
	created_at DESC
LIMIT
	sqlc.arg('limit')
OFFSET
	sqlc.arg('offset')
;

-- name: Create :exec
INSERT INTO
	activity_log_entries (
		actor,
		action,
		resource_type,
		resource_name,
		team_slug,
		environment,
		data
	)
VALUES
	(
		@actor,
		@action,
		@resource_type,
		@resource_name,
		@team_slug,
		@environment_name,
		@data
	)
;

-- name: Get :one
SELECT
	*
FROM
	activity_log_entries
WHERE
	id = @id
;

-- name: ListByIDs :many
SELECT
	*
FROM
	activity_log_entries
WHERE
	id = ANY (@ids::UUID[])
ORDER BY
	created_at DESC
;

-- name: CountForTeam :one
SELECT
	COUNT(*)
FROM
	activity_log_entries
WHERE
	team_slug = @team_slug
;

-- name: CountForResource :one
SELECT
	COUNT(*)
FROM
	activity_log_entries
WHERE
	resource_type = @resource_type
	AND resource_name = @resource_name
;

-- name: CountForTeamByResource :one
SELECT
	COUNT(*)
FROM
	activity_log_entries
WHERE
	team_slug = @team_slug
	AND resource_type = @resource_type
;
