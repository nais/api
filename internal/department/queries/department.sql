-- name: Create :one
INSERT INTO
	departments (slug, purpose, slack_channel)
VALUES
	(@slug, @purpose, @slack_channel)
RETURNING
	*
;

-- name: Count :one
SELECT
	COUNT(*)
FROM
	departments
;

-- name: List :many
SELECT
	*
FROM
	departments
ORDER BY 
	slug ASC
LIMIT
	sqlc.arg('limit')
OFFSET
	sqlc.arg('offset')
;
