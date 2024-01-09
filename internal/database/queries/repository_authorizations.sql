-- name: CreateRepositoryAuthorization :exec
INSERT INTO repository_authorizations (team_slug, github_repository, repository_authorization)
VALUES (@team_slug, @github_repository, @repository_authorization);

-- name: RemoveRepositoryAuthorization :exec
DELETE FROM repository_authorizations
WHERE
    team_slug = @team_slug
    AND github_repository = @github_repository
    AND repository_authorization = @repository_authorization;

-- name: GetRepositoryAuthorizations :many
SELECT
    repository_authorization
FROM
    repository_authorizations
WHERE
    team_slug = @team_slug
    AND github_repository = @github_repository
ORDER BY
    repository_authorization;
