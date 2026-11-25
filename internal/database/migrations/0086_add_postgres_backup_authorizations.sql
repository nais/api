-- +goose Up
INSERT INTO
	authorizations (name, description)
VALUES
	(
		'postgres:backups:read',
		'Permission to read standalone Postgres backups.'
	),
	(
		'postgres:backups:create',
		'Permission to request standalone Postgres backups.'
	),
	(
		'postgres:backups:delete',
		'Permission to request deletion of standalone Postgres backups.'
	)
;

INSERT INTO
	role_authorizations (role_name, authorization_name)
VALUES
	('Team member', 'postgres:backups:read'),
	('Team member', 'postgres:backups:create'),
	('Team member', 'postgres:backups:delete'),
	('Team owner', 'postgres:backups:read'),
	('Team owner', 'postgres:backups:create'),
	('Team owner', 'postgres:backups:delete')
;

-- +goose Down
DELETE FROM role_authorizations
WHERE
	authorization_name IN (
		'postgres:backups:read',
		'postgres:backups:create',
		'postgres:backups:delete'
	)
;

DELETE FROM authorizations
WHERE
	name IN (
		'postgres:backups:read',
		'postgres:backups:create',
		'postgres:backups:delete'
	)
;
