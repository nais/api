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
	"group",
	version,
	kind,
	name
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
