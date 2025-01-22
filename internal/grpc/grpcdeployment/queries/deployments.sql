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
