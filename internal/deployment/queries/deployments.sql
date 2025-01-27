-- name: ListByIDs :many
SELECT
	*
FROM
	deployments
WHERE
	id = ANY (@ids::UUID[])
ORDER BY
	created_at
;

-- name: ListDeploymentResourcesByIDs :many
SELECT
	*
FROM
	deployment_k8s_resources
WHERE
	id = ANY (@ids::UUID[])
ORDER BY
	created_at
;

-- name: ListDeploymentStatusesByIDs :many
SELECT
	*
FROM
	deployment_statuses
WHERE
	id = ANY (@ids::UUID[])
ORDER BY
	created_at
;

-- name: ListByTeamSlug :many
SELECT
	*
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

-- name: CountForTeam :one
SELECT
	COUNT(*)
FROM
	deployments
WHERE
	team_slug = @team_slug::slug
;

-- name: ListResourcesForDeployment :many
SELECT
	*
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

-- name: CountResourcesForDeployment :one
SELECT
	COUNT(*)
FROM
	deployment_k8s_resources
WHERE
	deployment_id = @deployment_id
;

-- name: ListStatusesForDeployment :many
SELECT
	*
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

-- name: CountStatusesForDeployment :one
SELECT
	COUNT(*)
FROM
	deployment_statuses
WHERE
	deployment_id = @deployment_id
;

-- name: ListForWorkload :many
SELECT
	deployments.*
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

-- name: CountForWorkload :one
SELECT
	COUNT(*)
FROM
	deployments
	JOIN deployment_k8s_resources ON deployments.id = deployment_k8s_resources.deployment_id
WHERE
	deployment_k8s_resources.name = @workload_name
	AND deployment_k8s_resources.kind = @workload_kind
	AND deployments.environment_name = @environment_name
	AND deployments.team_slug = @team_slug
;
