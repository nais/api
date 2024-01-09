-- name: CreateServiceAccount :one
INSERT INTO service_accounts (name)
VALUES (@name)
RETURNING *;

-- name: GetServiceAccounts :many
SELECT * FROM service_accounts
ORDER BY name ASC;

-- name: GetServiceAccountByName :one
SELECT * FROM service_accounts
WHERE name = @name;

-- name: GetServiceAccountByApiKey :one
SELECT service_accounts.* FROM api_keys
JOIN service_accounts ON service_accounts.id = api_keys.service_account_id
WHERE api_keys.api_key = @api_key;

-- name: DeleteServiceAccount :exec
DELETE FROM service_accounts
WHERE id = @id;

-- name: GetServiceAccountRoles :many
SELECT * FROM service_account_roles
WHERE service_account_id = @service_account_id
ORDER BY role_name ASC;
