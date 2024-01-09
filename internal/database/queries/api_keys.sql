-- name: CreateAPIKey :exec
INSERT INTO api_keys (api_key, service_account_id)
VALUES (@api_key, @service_account_id);

-- name: RemoveApiKeysFromServiceAccount :exec
DELETE FROM api_keys
WHERE service_account_id = @service_account_id;
