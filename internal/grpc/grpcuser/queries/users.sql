-- name: GetByID :one
SELECT
	*
FROM
	users
WHERE
	id = @id
;

-- name: GetByExternalID :one
SELECT
	*
FROM
	users
WHERE
	external_id = @external_id
;

-- name: GetByEmail :one
SELECT
	*
FROM
	users
WHERE
	email = LOWER(@email)
;

-- name: Count :one
SELECT
	COUNT(*)
FROM
	users
;

-- name: List :many
SELECT
	*
FROM
	users
ORDER BY
	name,
	email ASC
LIMIT
	sqlc.arg('limit')
OFFSET
	sqlc.arg('offset')
;
