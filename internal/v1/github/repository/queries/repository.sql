-- name: CountForTeam :one
SELECT
	COUNT(*)
FROM
	team_repositories
WHERE
	team_slug = @team_slug
	AND CASE
		WHEN sqlc.narg(search)::TEXT IS NOT NULL THEN github_repository ILIKE '%' || @search || '%'
		ELSE TRUE
	END
;

-- name: ListForTeam :many
SELECT
	*
FROM
	team_repositories
WHERE
	team_slug = @team_slug
	AND CASE
		WHEN sqlc.narg(search)::TEXT IS NOT NULL THEN github_repository ILIKE '%' || @search || '%'
		ELSE TRUE
	END
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

-- name: Remove :exec
DELETE FROM team_repositories
WHERE
	team_slug = @team_slug
	AND github_repository = @github_repository
;
