-- +goose Up
INSERT INTO
	authorizations (name, description)
VALUES
	(
		'postgres:access:read',
		'Permission to read personal Postgres access status and credentials'
	)
ON CONFLICT (name) DO NOTHING
;

INSERT INTO
	role_authorizations (role_name, authorization_name)
VALUES
	('Team member', 'postgres:access:read'),
	('Team owner', 'postgres:access:read')
ON CONFLICT (role_name, authorization_name) DO NOTHING
;

-- +goose Down
DELETE FROM role_authorizations
WHERE
	authorization_name = 'postgres:access:read'
;

DELETE FROM authorizations
WHERE
	name = 'postgres:access:read'
;
