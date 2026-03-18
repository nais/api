-- +goose Up
-- Create new authorizations for managing configs (ConfigMaps)
INSERT INTO
	authorizations (name, description)
VALUES
	(
		'teams:configs:create',
		'Permission to create configs for a team.'
	),
	(
		'teams:configs:update',
		'Permission to update configs for a team.'
	),
	(
		'teams:configs:delete',
		'Permission to delete configs for a team.'
	)
;

-- Grant these authorizations to Team member role
INSERT INTO
	role_authorizations (role_name, authorization_name)
VALUES
	('Team member', 'teams:configs:create'),
	('Team member', 'teams:configs:update'),
	('Team member', 'teams:configs:delete')
;

-- Grant these authorizations to Team owner role
INSERT INTO
	role_authorizations (role_name, authorization_name)
VALUES
	('Team owner', 'teams:configs:create'),
	('Team owner', 'teams:configs:update'),
	('Team owner', 'teams:configs:delete')
;
