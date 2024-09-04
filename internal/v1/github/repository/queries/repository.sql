-- name: CountForTeam :one
SELECT
	COUNT(*)
FROM
	team_repositories
WHERE
	team_slug = @team_slug
;

-- name: ListForTeam :many
SELECT
	*
FROM
	team_repositories
WHERE
	team_slug = @team_slug
ORDER BY
	github_repository ASC
LIMIT
	sqlc.arg('limit')
OFFSET
	sqlc.arg('offset')
;

-- name: Create :one
INSERT INTO
	team_repositories (team_slug, github_repository)
VALUES
	(@team_slug, @github_repository)
RETURNING
	*
;
