-- name: CreateDeployment :one
INSERT INTO
	deployments (
		external_id,
		created_at,
		team_slug,
		repository,
		environment_name
	)
VALUES
	(
		@external_id,
		COALESCE(@created_at, CLOCK_TIMESTAMP())::TIMESTAMPTZ,
		@team_slug,
		@repository,
		@environment_name
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
		(
			SELECT
				deployments.id
			FROM
				deployments
			WHERE
				deployments.id = @deployment_id
				OR deployments.external_id = @external_deployment_id
		),
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
		COALESCE(@created_at, CLOCK_TIMESTAMP())::TIMESTAMPTZ,
		(
			SELECT
				deployments.id
			FROM
				deployments
			WHERE
				deployments.id = @deployment_id
				OR deployments.external_id = @external_deployment_id
		),
		@state,
		@message
	)
RETURNING
	id
;
