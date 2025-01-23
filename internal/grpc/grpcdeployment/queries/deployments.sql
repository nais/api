-- name: CreateDeployment :one
INSERT INTO
	deployments (created_at, team_slug, repository, environment)
VALUES
	(
		COALESCE(@created_at, CLOCK_TIMESTAMP()),
		@team_slug,
		@repository,
		@environment
	)
RETURNING
	id
;

-- name: TeamExists :one
SELECT
	EXISTS (
		SELECT
			1
		FROM
			teams
		WHERE
			slug = @slug
	)
;

-- name: CreateDeploymentK8sResource :one
INSERT INTO
	deployment_k8s_resources (
		deployment_id,
		"group",
		version,
		kind,
		name,
		namespace
	)
VALUES
	(
		@deployment_id,
		sqlc.arg('group'),
		@version,
		@kind,
		@name,
		@namespace
	)
RETURNING
	id
;

-- name: CreateDeploymentStatus :one
INSERT INTO
	deployment_statuses (created_at, deployment_id, state, message)
VALUES
	(
		COALESCE(@created_at, CLOCK_TIMESTAMP()),
		@deployment_id,
		@state,
		@message
	)
RETURNING
	id
;
