-- name: ListForTeam :many
SELECT
	sqlc.embed(team_repositories),
	COUNT(*) OVER () AS total_count
FROM
	team_repositories
WHERE
	team_slug = @team_slug
	AND CASE
		WHEN sqlc.narg(search)::TEXT IS NOT NULL THEN github_repository ILIKE '%' || @search || '%'
		ELSE TRUE
	END
ORDER BY
	CASE
		WHEN @order_by::TEXT = 'name:asc' THEN github_repository
	END ASC,
	CASE
		WHEN @order_by::TEXT = 'name:desc' THEN github_repository
	END DESC,
	github_repository ASC
LIMIT
	sqlc.arg('limit')
OFFSET
	sqlc.arg('offset')
;

-- name: AddToTeam :one
INSERT INTO
	team_repositories (team_slug, github_repository)
VALUES
	(@team_slug, @github_repository)
RETURNING
	*
;

-- name: RemoveFromTeam :exec
DELETE FROM team_repositories
WHERE
	team_slug = @team_slug
	AND github_repository = @github_repository
;

-- name: GetByName :many
SELECT
	*
FROM
	team_repositories
WHERE
	github_repository = @github_repository
ORDER BY
	team_slug ASC
;
