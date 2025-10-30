-- +goose Up
INSERT INTO
	roles (name, description)
VALUES
	(
		'GitHub repository',
		'Granted to repositories linked to a team.'
	)
;

INSERT INTO
	role_authorizations (role_name, authorization_name)
VALUES
	('GitHub repository', 'valkeys:create'),
	('GitHub repository', 'valkeys:delete'),
	('GitHub repository', 'valkeys:update'),
	('GitHub repository', 'opensearches:create'),
	('GitHub repository', 'opensearches:delete'),
	('GitHub repository', 'opensearches:update')
;
