-- +goose Up
INSERT INTO
	authorizations (name, description)
VALUES
	(
		'postgres:delete',
		'Permission to delete Postgres instances.'
	)
;

INSERT INTO
	role_authorizations (role_name, authorization_name)
VALUES
	('Team member', 'postgres:delete'),
	('Team owner', 'postgres:delete')
;

-- +goose Down
DELETE FROM role_authorizations
WHERE
	authorization_name = 'postgres:delete'
;

DELETE FROM authorizations
WHERE
	name = 'postgres:delete'
;
