-- name: List :many
SELECT
	*
FROM
	service_accounts
ORDER BY
	name ASC
LIMIT
	sqlc.arg('limit')
OFFSET
	sqlc.arg('offset')
;

-- name: Count :one
SELECT
	COUNT(*)
FROM
	service_accounts
;

-- name: GetByToken :one
SELECT
	service_accounts.*
FROM
	service_account_tokens
	JOIN service_accounts ON service_accounts.id = service_account_tokens.service_account_id
WHERE
	service_account_tokens.token = @token
;

-- name: GetByName :one
SELECT
	*
FROM
	service_accounts
WHERE
	name = @name
;

-- name: Create :one
INSERT INTO
	service_accounts (name)
VALUES
	(@name)
RETURNING
	*
;

-- name: RemoveApiKeysFromServiceAccount :exec
DELETE FROM service_account_tokens
WHERE
	service_account_id = @service_account_id
;

-- name: CreateToken :exec
INSERT INTO
	service_account_tokens (expires_at, note, token, service_account_id)
VALUES
	(@expires_at, @note, @token, @service_account_id)
;

-- name: Delete :exec
DELETE FROM service_accounts
WHERE
	id = @id
;

-- name: GetByIDs :many
SELECT
	*
FROM
	service_accounts
WHERE
	id = ANY (@ids::UUID[])
ORDER BY
	name ASC
;
