-- +goose Up
-- Create new authorizations for elevations and reading secret values
INSERT INTO
	authorizations (name, description)
VALUES
	(
		'teams:elevations:create',
		'Permission to create elevations for team resources.'
	),
	(
		'teams:secrets:read-values',
		'Permission to read secret values (requires elevation).'
	)
;

-- Grant these authorizations to Team member role
INSERT INTO
	role_authorizations (role_name, authorization_name)
VALUES
	('Team member', 'teams:elevations:create'),
	('Team member', 'teams:secrets:read-values')
;

-- Grant these authorizations to Team owner role
INSERT INTO
	role_authorizations (role_name, authorization_name)
VALUES
	('Team owner', 'teams:elevations:create'),
	('Team owner', 'teams:secrets:read-values')
;
