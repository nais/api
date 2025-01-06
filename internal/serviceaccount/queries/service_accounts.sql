-- name: List :many
SELECT
	*
FROM
	service_accounts
ORDER BY
	name ASC
;

-- name: GetByApiKey :one
SELECT
	service_accounts.*
FROM
	api_keys
	JOIN service_accounts ON service_accounts.id = api_keys.service_account_id
WHERE
	api_keys.api_key = @api_key
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
DELETE FROM api_keys
WHERE
	service_account_id = @service_account_id
;

-- name: CreateAPIKey :exec
INSERT INTO
	api_keys (api_key, service_account_id)
VALUES
	(@api_key, @service_account_id)
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
