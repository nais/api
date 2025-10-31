-- +goose Up
DELETE FROM role_authorizations
WHERE
	role_name = 'GitHub repository'
	AND authorization_name = 'valkeys:delete'
;

DELETE FROM role_authorizations
WHERE
	role_name = 'GitHub repository'
	AND authorization_name = 'opensearches:delete'
;
