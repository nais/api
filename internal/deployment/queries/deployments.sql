-- name: ListByIDs :many
SELECT
	*
FROM
	deployments
WHERE
	id = ANY (@ids::UUID[])
ORDER BY
	created_at
;

-- name: ListByTeamSlug :many
SELECT
	*
FROM
	deployments
WHERE
	team_slug = @team_slug::slug
ORDER BY
	created_at DESC
LIMIT
	sqlc.arg('limit')
OFFSET
	sqlc.arg('offset')
;

-- name: CountForTeam :one
SELECT
	COUNT(*)
FROM
	deployments
WHERE
	team_slug = @team_slug::slug
;
