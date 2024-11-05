-- name: GetByApiKey :one
SELECT
	service_accounts.*
FROM
	api_keys
	JOIN service_accounts ON service_accounts.id = api_keys.service_account_id
WHERE
	api_keys.api_key = @api_key
;
