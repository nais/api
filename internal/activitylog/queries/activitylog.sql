-- name: ListForTeam :many
SELECT
	sqlc.embed(activity_log_combined_view),
	COUNT(*) OVER () AS total_count
FROM
	activity_log_combined_view
WHERE
	team_slug = @team_slug
	AND CASE
		WHEN sqlc.narg('filter')::TEXT[] [] IS NULL THEN TRUE
		ELSE sqlc.narg('filter')::TEXT[] [] @> ARRAY[resource_type, action]
	END
ORDER BY
	created_at DESC
LIMIT
	sqlc.arg('limit')
OFFSET
	sqlc.arg('offset')
;

-- name: ListForResource :many
SELECT
	sqlc.embed(activity_log_combined_view),
	COUNT(*) OVER () AS total_count
FROM
	activity_log_combined_view
WHERE
	resource_type = @resource_type
	AND resource_name = @resource_name
	AND CASE
		WHEN sqlc.narg('filter')::TEXT[] [] IS NULL THEN TRUE
		ELSE sqlc.narg('filter')::TEXT[] [] @> ARRAY[resource_type, action]
	END
ORDER BY
	created_at DESC
LIMIT
	sqlc.arg('limit')
OFFSET
	sqlc.arg('offset')
;

-- name: ListForResourceTeamAndEnvironment :many
SELECT
	sqlc.embed(activity_log_combined_view),
	COUNT(*) OVER () AS total_count
FROM
	activity_log_combined_view
WHERE
	resource_type = @resource_type
	AND team_slug = @team_slug
	AND resource_name = @resource_name
	AND environment = @environment_name
	AND CASE
		WHEN sqlc.narg('filter')::TEXT[] [] IS NULL THEN TRUE
		ELSE sqlc.narg('filter')::TEXT[] [] @> ARRAY[resource_type, action]
	END
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
	activity_log_combined_view
WHERE
	id = @id
;

-- name: ListByIDs :many
SELECT
	*
FROM
	activity_log_combined_view
WHERE
	id = ANY (@ids::UUID[])
ORDER BY
	created_at DESC
;

-- name: RefreshMaterializedView :exec
REFRESH MATERIALIZED VIEW CONCURRENTLY activity_log_subset_mat_view
;
