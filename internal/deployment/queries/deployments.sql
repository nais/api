-- name: List :many
SELECT
	sqlc.embed(deployments),
	COUNT(*) OVER () AS total_count
FROM
	deployments
WHERE
	(
		@since::TIMESTAMPTZ IS NULL
		OR created_at >= @since::TIMESTAMPTZ
	)
	AND team_slug != 'nais-verification'
	AND (
		sqlc.narg('environments')::TEXT[] IS NULL
		OR environment_name = ANY (sqlc.narg('environments')::TEXT[])
	)
ORDER BY
	CASE
		WHEN @order_by::TEXT = 'asc' THEN created_at
	END ASC,
	CASE
		WHEN @order_by::TEXT = 'desc' THEN created_at
	END DESC
LIMIT
	sqlc.arg('limit')
OFFSET
	sqlc.arg('offset')
;

-- name: ListByIDs :many
SELECT
	*
FROM
	deployments
WHERE
	id = ANY (@ids::UUID[])
ORDER BY
	id
;

-- name: ListDeploymentResourcesByIDs :many
SELECT
	*
FROM
	deployment_k8s_resources
WHERE
	id = ANY (@ids::UUID[])
ORDER BY
	id
;

-- name: ListDeploymentStatusesByIDs :many
SELECT
	*
FROM
	deployment_statuses
WHERE
	id = ANY (@ids::UUID[])
ORDER BY
	id
;

-- name: ListByTeamSlug :many
SELECT
	sqlc.embed(deployments),
	COUNT(*) OVER () AS total_count
FROM
	deployments
WHERE
	team_slug = @team_slug::slug
ORDER BY
	created_at DESC
LIMIT
	sqlc.arg('limit')
OFFSET
	sqlc.arg('offset')
;

-- name: ListResourcesForDeployment :many
SELECT
	sqlc.embed(deployment_k8s_resources),
	COUNT(*) OVER () AS total_count
FROM
	deployment_k8s_resources
WHERE
	deployment_id = @deployment_id
ORDER BY
	created_at DESC
LIMIT
	sqlc.arg('limit')
OFFSET
	sqlc.arg('offset')
;

-- name: ListStatusesForDeployment :many
SELECT
	sqlc.embed(deployment_statuses),
	COUNT(*) OVER () AS total_count
FROM
	deployment_statuses
WHERE
	deployment_id = @deployment_id
ORDER BY
	created_at DESC
LIMIT
	sqlc.arg('limit')
OFFSET
	sqlc.arg('offset')
;

-- name: ListForWorkload :many
SELECT
	sqlc.embed(deployments),
	COUNT(*) OVER () AS total_count
FROM
	deployments
	JOIN deployment_k8s_resources ON deployments.id = deployment_k8s_resources.deployment_id
WHERE
	deployment_k8s_resources.name = @workload_name
	AND deployment_k8s_resources.kind = @workload_kind
	AND deployments.environment_name = @environment_name
	AND deployments.team_slug = @team_slug
ORDER BY
	deployments.created_at DESC
LIMIT
	sqlc.arg('limit')
OFFSET
	sqlc.arg('offset')
;

-- name: LatestDeploymentTimestampForWorkload :one
SELECT
	deployments.created_at
FROM
	deployments
	JOIN deployment_k8s_resources ON deployments.id = deployment_k8s_resources.deployment_id
WHERE
	deployment_k8s_resources.name = @workload_name
	AND deployment_k8s_resources.kind = @workload_kind
	AND deployments.environment_name = @environment_name
	AND deployments.team_slug = @team_slug
ORDER BY
	deployments.created_at DESC
LIMIT
	1
;

-- name: CleanupNaisVerification :execresult
DELETE FROM deployments
WHERE
	team_slug = 'nais-verification'
	AND created_at < NOW() - '1 week'::INTERVAL
;
