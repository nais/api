-- +goose Up
DELETE FROM role_authorizations
WHERE
	role_name = 'Team member'
	AND authorization_name IN (
		'service_accounts:create',
		'service_accounts:delete',
		'service_accounts:update'
	)
;
