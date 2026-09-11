-- name: ListForTeam :many
WITH
	matching_entries AS (
		SELECT
			COUNT(*) AS total_count
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
	)
SELECT
	sqlc.embed(activity_log_combined_view),
	matching_entries.total_count
FROM
	activity_log_combined_view
	CROSS JOIN matching_entries
WHERE
	activity_log_combined_view.team_slug = @team_slug
	AND (
		sqlc.narg('filter')::TEXT[] IS NULL
		OR (
			activity_log_combined_view.resource_type || ':' || activity_log_combined_view.action
		) = ANY (sqlc.narg('filter')::TEXT[])
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
		OR activity_log_combined_view.created_at >= sqlc.narg('from')::TIMESTAMPTZ
	)
	AND (
		sqlc.narg('to')::TIMESTAMPTZ IS NULL
		OR activity_log_combined_view.created_at < sqlc.narg('to')::TIMESTAMPTZ
	)
ORDER BY
	activity_log_combined_view.created_at DESC
LIMIT
	sqlc.arg('limit')
OFFSET
	sqlc.arg('offset')
;

-- name: ListForTenant :many
WITH
	matching_entries AS (
		SELECT
			COUNT(*) AS total_count
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
	)
SELECT
	sqlc.embed(activity_log_combined_view),
	matching_entries.total_count
FROM
	activity_log_combined_view
	CROSS JOIN matching_entries
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
WITH
	matching_entries AS (
		SELECT
			COUNT(*) AS total_count
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
	)
SELECT
	sqlc.embed(activity_log_combined_view),
	matching_entries.total_count
FROM
	activity_log_combined_view
	CROSS JOIN matching_entries
WHERE
	activity_log_combined_view.resource_type = @resource_type
	AND activity_log_combined_view.resource_name = @resource_name
	AND (
		sqlc.narg('filter')::TEXT[] IS NULL
		OR (
			activity_log_combined_view.resource_type || ':' || activity_log_combined_view.action
		) = ANY (sqlc.narg('filter')::TEXT[])
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
		OR activity_log_combined_view.created_at >= sqlc.narg('from')::TIMESTAMPTZ
	)
	AND (
		sqlc.narg('to')::TIMESTAMPTZ IS NULL
		OR activity_log_combined_view.created_at < sqlc.narg('to')::TIMESTAMPTZ
	)
ORDER BY
	activity_log_combined_view.created_at DESC
LIMIT
	sqlc.arg('limit')
OFFSET
	sqlc.arg('offset')
;

-- name: ListForResourceAndTeam :many
SELECT
	sqlc.embed(activity_log_combined_view),
	COUNT(*) OVER () AS total_count
FROM
	activity_log_combined_view
WHERE
	resource_type = @resource_type
	AND resource_name = @resource_name
	AND team_slug = @team_slug
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

-- name: ListForResourceWithoutTeam :many
SELECT
	sqlc.embed(activity_log_combined_view),
	COUNT(*) OVER () AS total_count
FROM
	activity_log_combined_view
WHERE
	resource_type = @resource_type
	AND resource_name = @resource_name
	AND team_slug IS NULL
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

-- name: FacetsForTeam :many
SELECT
	resource_type,
	action,
	COALESCE(environment, '') AS environment,
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
	team_slug = @team_slug
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
	environment
ORDER BY
	resource_type,
	action,
	environment
;

-- name: FacetsForResource :many
SELECT
	resource_type,
	action,
	COALESCE(environment, '') AS environment,
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
	resource_type = @resource_type
	AND resource_name = @resource_name
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
	environment
ORDER BY
	resource_type,
	action,
	environment
;

-- name: FacetsForResourceAndTeam :many
SELECT
	resource_type,
	action,
	COALESCE(environment, '') AS environment,
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
	resource_type = @resource_type
	AND resource_name = @resource_name
	AND team_slug = @team_slug
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
	environment
ORDER BY
	resource_type,
	action,
	environment
;

-- name: FacetsForResourceWithoutTeam :many
SELECT
	resource_type,
	action,
	COALESCE(environment, '') AS environment,
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
	resource_type = @resource_type
	AND resource_name = @resource_name
	AND team_slug IS NULL
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
	environment
ORDER BY
	resource_type,
	action,
	environment
;

-- name: FacetsForResourceTeamAndEnvironment :many
SELECT
	resource_type,
	action,
	COALESCE(environment, '') AS environment,
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
	resource_type = @resource_type
	AND resource_name = @resource_name
	AND team_slug = @team_slug
	AND environment = @environment_name
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
	environment
ORDER BY
	resource_type,
	action,
	environment
;

-- name: FacetsForTenantActivityTypes :many
SELECT
	resource_type,
	action,
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
	environment
ORDER BY
	resource_type,
	action,
	environment
;

-- name: RefreshMaterializedView :exec
REFRESH MATERIALIZED VIEW CONCURRENTLY activity_log_subset_mat_view
;
