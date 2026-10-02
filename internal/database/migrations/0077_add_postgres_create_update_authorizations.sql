-- +goose Up
INSERT INTO
	authorizations (name, description)
VALUES
	(
		'postgres:create',
		'Permission to create Postgres instances.'
	),
	(
		'postgres:update',
		'Permission to update Postgres instances.'
	)
;

INSERT INTO
	role_authorizations (role_name, authorization_name)
VALUES
	('Team member', 'postgres:create'),
	('Team member', 'postgres:update'),
	('Team owner', 'postgres:create'),
	('Team owner', 'postgres:update')
;

-- +goose Down
DELETE FROM role_authorizations
WHERE
	authorization_name IN ('postgres:create', 'postgres:update')
;

DELETE FROM authorizations
WHERE
	name IN ('postgres:create', 'postgres:update')
;
