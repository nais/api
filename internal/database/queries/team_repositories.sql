-- name: GetTeamRepositories :many
SELECT
    github_repository
FROM
    team_repositories
WHERE
	team_slug = @team_slug
ORDER BY
    github_repository ASC;

-- name: IsTeamRepository :one
SELECT
	COUNT(*) > 0
FROM
	team_repositories
WHERE
	team_slug = @team_slug
	AND github_repository = @github_repository;
