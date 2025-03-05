-- name: List :many
SELECT
	*
FROM
	environments
ORDER BY
	name
;

-- name: ListByNames :many
SELECT
	*
FROM
	environments
WHERE
	name = ANY (@names::TEXT[])
ORDER BY
	name
;

-- name: Get :one
SELECT
	*
FROM
	environments
WHERE
	name = @name
;

-- name: DeleteAllEnvironments :exec
DELETE FROM environments
;

-- name: InsertEnvironment :exec
INSERT INTO
	environments (name, gcp)
VALUES
	(@name, @gcp)
;
