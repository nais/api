-- +goose Up
INSERT INTO
	authorizations (name, description)
VALUES
	(
		'postgres:access:grant',
		'Permission to grant access to a Postgres cluster'
	)
;

INSERT INTO
	role_authorizations (role_name, authorization_name)
VALUES
	('Team member', 'postgres:access:grant'),
	('Team owner', 'postgres:access:grant')
;
