-- name: List :many
SELECT
	sqlc.embed(service_accounts),
	COUNT(*) OVER () AS total_count
FROM
	service_accounts
ORDER BY
	name,
	team_slug
LIMIT
	sqlc.arg('limit')
OFFSET
	sqlc.arg('offset')
;

-- name: GetServiceAccountAndTokenBySecret :one
SELECT
	sqlc.embed(service_accounts),
	sqlc.embed(service_account_tokens)
FROM
	service_account_tokens
	JOIN service_accounts ON service_accounts.id = service_account_tokens.service_account_id
WHERE
	service_account_tokens.token = @token
	AND (
		service_account_tokens.expires_at IS NULL
		OR service_account_tokens.expires_at >= CURRENT_DATE
	)
;

-- name: ListTokensForServiceAccount :many
SELECT
	sqlc.embed(service_account_tokens),
	COUNT(*) OVER () AS total_count
FROM
	service_account_tokens
WHERE
	service_account_id = @service_account_id
ORDER BY
	name
LIMIT
	sqlc.arg('limit')
OFFSET
	sqlc.arg('offset')
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
	service_accounts (name, description, team_slug)
VALUES
	(@name, @description, @team_slug)
RETURNING
	*
;

-- name: Update :one
UPDATE service_accounts
SET
	description = COALESCE(sqlc.narg('description'), description)
WHERE
	id = @id
RETURNING
	*
;

-- name: RemoveApiKeysFromServiceAccount :exec
DELETE FROM service_account_tokens
WHERE
	service_account_id = @service_account_id
;

-- name: CreateToken :one
INSERT INTO
	service_account_tokens (
		expires_at,
		name,
		description,
		token,
		service_account_id
	)
VALUES
	(
		@expires_at,
		@name,
		@description,
		@token,
		@service_account_id
	)
RETURNING
	*
;

-- name: UpdateToken :one
UPDATE service_account_tokens
SET
	name = COALESCE(sqlc.narg('name'), name),
	description = COALESCE(sqlc.narg('description'), description)
WHERE
	id = @id
RETURNING
	*
;

-- name: Delete :exec
DELETE FROM service_accounts
WHERE
	id = @id
;

-- name: DeleteToken :exec
DELETE FROM service_account_tokens
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

-- name: GetTokensByIDs :many
SELECT
	*
FROM
	service_account_tokens
WHERE
	id = ANY (@ids::UUID[])
ORDER BY
	created_at
;

-- name: UpdateTokenLastUsedAt :exec
UPDATE service_account_tokens
SET
	last_used_at = CLOCK_TIMESTAMP()
WHERE
	id = @id
;

-- name: LastUsedAt :one
SELECT
	MAX(last_used_at)::TIMESTAMPTZ AS last_used_at
FROM
	service_account_tokens
WHERE
	service_account_id = @service_account_id
;

-- TODO: Remove once static service accounts has been removed
-- name: DeleteStaticServiceAccounts :exec
DELETE FROM service_accounts
WHERE
	name LIKE 'nais-%'
;
