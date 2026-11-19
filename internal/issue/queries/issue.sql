-- name: GetIssueByID :one
SELECT
	*
FROM
	issues
WHERE
	id = @id
;

-- name: GetSeverityScoreForWorkload :one
SELECT
	SUM(
		CASE
			WHEN severity = 'CRITICAL'::severity_level THEN 10000
			WHEN severity = 'WARNING'::severity_level THEN 100
			WHEN severity = 'TODO'::severity_level THEN 1
			ELSE 0
		END
	) AS severity_score
FROM
	issues
WHERE
	resource_name = @resource_name
	AND resource_type = @resource_type
	AND env = @env
	AND team = @team
;

-- name: ListIssues :many
SELECT
	*,
	COUNT(*) OVER () AS total_count
FROM
	issues
WHERE
	(
		@team::TEXT = ''
		OR team = @team
	)
	AND (
		sqlc.narg('team_slugs')::slug[] IS NULL
		OR team = ANY (sqlc.narg('team_slugs')::slug[])
	)
	AND (
		sqlc.narg('env')::TEXT[] IS NULL
		OR env = ANY (sqlc.narg('env')::TEXT[])
	)
	AND (
		sqlc.narg('issue_type')::TEXT IS NULL
		OR CASE
			WHEN issue_type = 'POSTGRES_BRANCH' THEN 'POSTGRES'
			ELSE issue_type
		END = sqlc.narg('issue_type')::TEXT
	)
	AND (
		sqlc.narg('severity')::severity_level IS NULL
		OR severity = sqlc.narg('severity')::severity_level
	)
	AND (
		sqlc.narg('resource_type')::TEXT IS NULL
		OR CASE
			WHEN resource_type = 'POSTGRES_BRANCH' THEN 'POSTGRES'
			ELSE resource_type
		END = sqlc.narg('resource_type')::TEXT
	)
	AND (
		sqlc.narg('resource_name')::TEXT IS NULL
		OR resource_name = sqlc.narg('resource_name')::TEXT
		OR (
			resource_type = 'POSTGRES_BRANCH'
			AND resource_name = sqlc.narg('legacy_resource_name')::TEXT
		)
	)
ORDER BY
	CASE
		WHEN @order_by::TEXT = 'env:asc' THEN env
	END ASC,
	CASE
		WHEN @order_by::TEXT = 'env:desc' THEN env
	END DESC,
	CASE
		WHEN @order_by::TEXT = 'issue_type:asc' THEN CASE
			WHEN issue_type = 'POSTGRES_BRANCH' THEN 'POSTGRES'
			ELSE issue_type
		END
	END ASC,
	CASE
		WHEN @order_by::TEXT = 'issue_type:desc' THEN CASE
			WHEN issue_type = 'POSTGRES_BRANCH' THEN 'POSTGRES'
			ELSE issue_type
		END
	END DESC,
	CASE
		WHEN @order_by::TEXT = 'resource_type:asc' THEN CASE
			WHEN resource_type = 'POSTGRES_BRANCH' THEN 'POSTGRES'
			ELSE resource_type
		END
	END ASC,
	CASE
		WHEN @order_by::TEXT = 'resource_type:desc' THEN CASE
			WHEN resource_type = 'POSTGRES_BRANCH' THEN 'POSTGRES'
			ELSE resource_type
		END
	END DESC,
	CASE
		WHEN @order_by::TEXT = 'resource_name:asc' THEN resource_name
	END ASC,
	CASE
		WHEN @order_by::TEXT = 'resource_name:desc' THEN resource_name
	END DESC,
	CASE
		WHEN @order_by::TEXT = 'severity:asc' THEN severity
	END ASC,
	CASE
		WHEN @order_by::TEXT = 'severity:desc' THEN severity
	END DESC,
	severity DESC,
	id
OFFSET
	sqlc.arg('offset')
LIMIT
	sqlc.arg('limit')
;

-- name: FacetsForIssues :many
-- Normalize before grouping, including zero-count facets. Keep ListIssues' stored
-- row types intact so legacy physical names still reach the Postgres read adapter.
WITH
	issue_rows AS (
		SELECT
			team,
			severity,
			env,
			resource_name,
			(
				CASE
					WHEN resource_type = 'POSTGRES_BRANCH' THEN 'POSTGRES'
					ELSE resource_type
				END
			)::TEXT AS resource_type,
			(
				CASE
					WHEN issue_type = 'POSTGRES_BRANCH' THEN 'POSTGRES'
					ELSE issue_type
				END
			)::TEXT AS issue_type,
			resource_type = 'POSTGRES_BRANCH' AS legacy_postgres
		FROM
			issues
	)
SELECT
	team,
	EXISTS (
		SELECT
			1
		FROM
			teams
		WHERE
			teams.slug = issues.team
	) AS team_exists,
	severity,
	resource_type,
	env,
	issue_type,
	COUNT(*) FILTER (
		WHERE
			(
				sqlc.narg('env')::TEXT[] IS NULL
				OR env = ANY (sqlc.narg('env')::TEXT[])
			)
			AND (
				sqlc.narg('issue_type')::TEXT IS NULL
				OR issue_type = sqlc.narg('issue_type')::TEXT
			)
			AND (
				sqlc.narg('severity')::severity_level IS NULL
				OR severity = sqlc.narg('severity')::severity_level
			)
			AND (
				sqlc.narg('filter_resource_type')::TEXT IS NULL
				OR resource_type = sqlc.narg('filter_resource_type')::TEXT
			)
			AND (
				sqlc.narg('filter_resource_name')::TEXT IS NULL
				OR resource_name = sqlc.narg('filter_resource_name')::TEXT
				OR (
					legacy_postgres
					AND resource_name = sqlc.narg('legacy_filter_resource_name')::TEXT
				)
			)
	) AS filtered_count
FROM
	issue_rows AS issues
WHERE
	(
		@team::TEXT = ''
		OR team = @team
	)
	AND (
		sqlc.narg('team_slugs')::slug[] IS NULL
		OR team = ANY (sqlc.narg('team_slugs')::slug[])
	)
	AND (
		sqlc.narg('scope_resource_type')::TEXT IS NULL
		OR resource_type = sqlc.narg('scope_resource_type')::TEXT
	)
	AND (
		sqlc.narg('scope_resource_name')::TEXT IS NULL
		OR resource_name = sqlc.narg('scope_resource_name')::TEXT
		OR (
			legacy_postgres
			AND resource_name = sqlc.narg('legacy_scope_resource_name')::TEXT
		)
	)
	AND (
		sqlc.narg('scope_env')::TEXT IS NULL
		OR env = sqlc.narg('scope_env')::TEXT
	)
GROUP BY
	team,
	severity,
	resource_type,
	env,
	issue_type
ORDER BY
	team,
	severity,
	resource_type,
	env,
	issue_type
;
