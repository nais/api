-- name: AddTeamRepository :exec
INSERT INTO team_repositories (team_slug, github_repository)
VALUES (@team_slug, @github_repository);

-- name: RemoveTeamRepository :exec
DELETE FROM team_repositories
WHERE
    team_slug = @team_slug
    AND github_repository = @github_repository;

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
