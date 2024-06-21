-- name: CreateRepositoryAuthorization :exec
INSERT INTO repository_authorizations (team_slug, github_repository)
VALUES (@team_slug, @github_repository);

-- name: RemoveRepositoryAuthorization :exec
DELETE FROM repository_authorizations
WHERE
    team_slug = @team_slug
    AND github_repository = @github_repository;

-- name: GetAuthorizedRepositories :many
SELECT
	github_repository
FROM
	repository_authorizations
WHERE
	team_slug = @team_slug
ORDER BY
	github_repository ASC;

-- name: IsRepositoryAuthorized :one
SELECT
	COUNT(*) > 0
FROM
	repository_authorizations
WHERE
	team_slug = @team_slug
	AND github_repository = @github_repository;
