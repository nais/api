-- +goose Up
-- Add Team viewer role with read-only permissions (cannot elevate)
INSERT INTO
	roles (name, description, is_only_global)
VALUES
	(
		'Team viewer',
		'Permits the actor to view team resources. Cannot modify resources or elevate privileges.',
		FALSE
	)
;

-- Add elevation:create authorization
INSERT INTO
	authorizations (name, description)
VALUES
	(
		'elevation:create',
		'Permission to create temporary privilege elevations'
	)
;

-- Grant elevation:create to Team member and Team owner (NOT Team viewer)
INSERT INTO
	role_authorizations (role_name, authorization_name)
VALUES
	('Team member', 'elevation:create'),
	('Team owner', 'elevation:create')
;

-- Team viewer gets read-only permissions
INSERT INTO
	role_authorizations (role_name, authorization_name)
VALUES
	('Team viewer', 'teams:secrets:list'),
	('Team viewer', 'service_accounts:read')
;
