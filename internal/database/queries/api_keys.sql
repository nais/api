-- name: CreateAPIKey :exec
INSERT INTO api_keys (api_key, service_account_id)
VALUES ($1, $2);

-- name: RemoveApiKeysFromServiceAccount :exec
DELETE FROM api_keys
WHERE service_account_id = $1;