-- name: CreateDeployment :one
INSERT INTO
	deployments (
		created_at,
		team_slug,
		github_repository,
		environment
	)
VALUES
	(
		@created_at,
		@team_slug,
		@github_repository,
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
