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
	team = @team
	AND (
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
		sqlc.narg('resource_type')::TEXT IS NULL
		OR resource_type = sqlc.narg('resource_type')::TEXT
	)
	AND (
		sqlc.narg('resource_name')::TEXT IS NULL
		OR resource_name = sqlc.narg('resource_name')::TEXT
	)
ORDER BY
	CASE
		WHEN @order_by::TEXT = 'env:asc' THEN env
	END ASC,
	CASE
		WHEN @order_by::TEXT = 'env:desc' THEN env
	END DESC,
	CASE
		WHEN @order_by::TEXT = 'issue_type:asc' THEN issue_type
	END ASC,
	CASE
		WHEN @order_by::TEXT = 'issue_type:desc' THEN issue_type
	END DESC,
	CASE
		WHEN @order_by::TEXT = 'resource_type:asc' THEN resource_type
	END ASC,
	CASE
		WHEN @order_by::TEXT = 'resource_type:desc' THEN resource_type
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
