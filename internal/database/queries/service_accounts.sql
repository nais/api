-- name: CreateServiceAccount :one
INSERT INTO service_accounts (name)
VALUES ($1)
RETURNING *;

-- name: GetServiceAccounts :many
SELECT * FROM service_accounts
ORDER BY name ASC;

-- name: GetServiceAccountByName :one
SELECT * FROM service_accounts
WHERE name = $1;

-- name: GetServiceAccountByApiKey :one
SELECT service_accounts.* FROM api_keys
JOIN service_accounts ON service_accounts.id = api_keys.service_account_id
WHERE api_keys.api_key = $1;

-- name: DeleteServiceAccount :exec
DELETE FROM service_accounts
WHERE id = $1;

-- name: GetServiceAccountRoles :many
SELECT * FROM service_account_roles
WHERE service_account_id = $1;