-- name: ListForTeam :many
SELECT
	sqlc.embed(activity_log_combined_view),
	COUNT(*) OVER () AS total_count
FROM
	activity_log_combined_view
WHERE
	team_slug = @team_slug
	AND (
		sqlc.narg('filter')::TEXT[] IS NULL
		OR (resource_type || ':' || action) = ANY (sqlc.narg('filter')::TEXT[])
	)
	AND (
		sqlc.narg('resource_types')::TEXT[] IS NULL
		OR resource_type = ANY (sqlc.narg('resource_types')::TEXT[])
	)
	AND (
		sqlc.narg('environments')::TEXT[] IS NULL
		OR environment = ANY (sqlc.narg('environments')::TEXT[])
	)
	AND (
		sqlc.narg('from')::TIMESTAMPTZ IS NULL
		OR created_at >= sqlc.narg('from')::TIMESTAMPTZ
	)
	AND (
		sqlc.narg('to')::TIMESTAMPTZ IS NULL
		OR created_at < sqlc.narg('to')::TIMESTAMPTZ
	)
ORDER BY
	created_at DESC
LIMIT
	sqlc.arg('limit')
OFFSET
	sqlc.arg('offset')
;

-- name: ListForTenant :many
SELECT
	sqlc.embed(activity_log_combined_view),
	COUNT(*) OVER () AS total_count
FROM
	activity_log_combined_view
WHERE
	(
		sqlc.narg('filter')::TEXT[] IS NULL
		OR (resource_type || ':' || action) = ANY (sqlc.narg('filter')::TEXT[])
	)
	AND (
		sqlc.narg('resource_types')::TEXT[] IS NULL
		OR resource_type = ANY (sqlc.narg('resource_types')::TEXT[])
	)
	AND (
		sqlc.narg('environments')::TEXT[] IS NULL
		OR environment = ANY (sqlc.narg('environments')::TEXT[])
	)
	AND (
		sqlc.narg('from')::TIMESTAMPTZ IS NULL
		OR created_at >= sqlc.narg('from')::TIMESTAMPTZ
	)
	AND (
		sqlc.narg('to')::TIMESTAMPTZ IS NULL
		OR created_at < sqlc.narg('to')::TIMESTAMPTZ
	)
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
	AND (
		sqlc.narg('filter')::TEXT[] IS NULL
		OR (resource_type || ':' || action) = ANY (sqlc.narg('filter')::TEXT[])
	)
	AND (
		sqlc.narg('resource_types')::TEXT[] IS NULL
		OR resource_type = ANY (sqlc.narg('resource_types')::TEXT[])
	)
	AND (
		sqlc.narg('environments')::TEXT[] IS NULL
		OR environment = ANY (sqlc.narg('environments')::TEXT[])
	)
	AND (
		sqlc.narg('from')::TIMESTAMPTZ IS NULL
		OR created_at >= sqlc.narg('from')::TIMESTAMPTZ
	)
	AND (
		sqlc.narg('to')::TIMESTAMPTZ IS NULL
		OR created_at < sqlc.narg('to')::TIMESTAMPTZ
	)
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
	AND (
		sqlc.narg('filter')::TEXT[] IS NULL
		OR (resource_type || ':' || action) = ANY (sqlc.narg('filter')::TEXT[])
	)
	AND (
		sqlc.narg('resource_types')::TEXT[] IS NULL
		OR activity_log_combined_view.resource_type = ANY (sqlc.narg('resource_types')::TEXT[])
	)
	AND (
		sqlc.narg('environments')::TEXT[] IS NULL
		OR activity_log_combined_view.environment = ANY (sqlc.narg('environments')::TEXT[])
	)
	AND (
		sqlc.narg('from')::TIMESTAMPTZ IS NULL
		OR created_at >= sqlc.narg('from')::TIMESTAMPTZ
	)
	AND (
		sqlc.narg('to')::TIMESTAMPTZ IS NULL
		OR created_at < sqlc.narg('to')::TIMESTAMPTZ
	)
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

-- name: Facets :many
SELECT
	resource_type,
	action,
	COALESCE(team_slug, '') AS team_slug,
	COALESCE(environment, '') AS environment,
	COUNT(*) AS total_count,
	COUNT(*) FILTER (
		WHERE
			(
				sqlc.narg('filter')::TEXT[] IS NULL
				OR (resource_type || ':' || action) = ANY (sqlc.narg('filter')::TEXT[])
			)
			AND (
				sqlc.narg('filter_resource_types')::TEXT[] IS NULL
				OR resource_type = ANY (sqlc.narg('filter_resource_types')::TEXT[])
			)
			AND (
				sqlc.narg('filter_environments')::TEXT[] IS NULL
				OR environment = ANY (sqlc.narg('filter_environments')::TEXT[])
			)
			AND (
				sqlc.narg('filter_from')::TIMESTAMPTZ IS NULL
				OR created_at >= sqlc.narg('filter_from')::TIMESTAMPTZ
			)
			AND (
				sqlc.narg('filter_to')::TIMESTAMPTZ IS NULL
				OR created_at < sqlc.narg('filter_to')::TIMESTAMPTZ
			)
	) AS filtered_count
FROM
	activity_log_combined_view
WHERE
	(
		sqlc.narg('team_slug')::TEXT IS NULL
		OR team_slug = sqlc.narg('team_slug')
	)
	AND (
		sqlc.narg('resource_type')::TEXT IS NULL
		OR resource_type = sqlc.narg('resource_type')
	)
	AND (
		sqlc.narg('resource_name')::TEXT IS NULL
		OR resource_name = sqlc.narg('resource_name')
	)
	AND (
		sqlc.narg('environment_name')::TEXT IS NULL
		OR environment = sqlc.narg('environment_name')
	)
	AND (
		sqlc.narg('from')::TIMESTAMPTZ IS NULL
		OR created_at >= sqlc.narg('from')::TIMESTAMPTZ
	)
	AND (
		sqlc.narg('to')::TIMESTAMPTZ IS NULL
		OR created_at < sqlc.narg('to')::TIMESTAMPTZ
	)
GROUP BY
	resource_type,
	action,
	team_slug,
	environment
ORDER BY
	resource_type,
	action,
	team_slug,
	environment
;

-- name: RefreshMaterializedView :exec
REFRESH MATERIALIZED VIEW CONCURRENTLY activity_log_subset_mat_view
;
