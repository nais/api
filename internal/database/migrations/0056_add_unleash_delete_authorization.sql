-- +goose Up
INSERT INTO
	authorizations (name, description)
VALUES
	(
		'unleash:delete',
		'Permission to delete Unleash instances.'
	)
;

INSERT INTO
	role_authorizations (role_name, authorization_name)
VALUES
	('Team member', 'unleash:delete'),
	('Team owner', 'unleash:delete')
;
