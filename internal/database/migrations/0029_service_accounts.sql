-- +goose Up
ALTER TABLE users
ADD COLUMN admin BOOLEAN NOT NULL DEFAULT FALSE
;

UPDATE users
SET
	admin = TRUE
WHERE
	(
		SELECT
			COUNT(*)
		FROM
			user_roles
		WHERE
			user_id = users.id
			AND role_name = 'Admin'
	) > 0
;

ALTER TABLE user_roles
DROP COLUMN target_service_account_id
;

DROP TABLE service_account_roles
;

DROP TABLE api_keys
;

DROP TABLE service_accounts
;

CREATE TABLE roles (
	name TEXT PRIMARY KEY,
	description TEXT NOT NULL,
	is_only_global BOOLEAN NOT NULL DEFAULT FALSE
)
;

COMMENT ON COLUMN roles.is_only_global IS 'If true, the role can only be assigned globally'
;

CREATE TABLE authorizations (name TEXT PRIMARY KEY, description TEXT NOT NULL)
;

INSERT INTO
	authorizations (name, description)
VALUES
	(
		'activity_logs:read',
		'Permission to read activity logs.'
	),
	(
		'service_accounts:create',
		'Permission to create service accounts.'
	),
	(
		'service_accounts:delete',
		'Permission to delete service accounts.'
	),
	(
		'service_accounts:read',
		'Permission to read service accounts.'
	),
	(
		'service_accounts:update',
		'Permission to update service accounts.'
	),
	('teams:create', 'Permission to create teams.'),
	('teams:delete', 'Permission to delete teams.'),
	(
		'teams:metadata:update',
		'Permission to update team metadata.'
	),
	(
		'teams:members:admin',
		'Permission to administer team members.'
	),
	(
		'teams:secrets:create',
		'Permission to create team secrets.'
	),
	(
		'teams:secrets:delete',
		'Permission to delete team secrets.'
	),
	(
		'teams:secrets:update',
		'Permission to update team secrets.'
	),
	(
		'teams:secrets:read',
		'Permission to read team secrets.'
	),
	(
		'teams:secrets:list',
		'Permission to list team secrets.'
	),
	(
		'repositories:create',
		'Permission to create team repositories.'
	),
	(
		'repositories:delete',
		'Permission to delete team repositories.'
	),
	(
		'applications:update',
		'Permission to update applications.'
	),
	(
		'applications:delete',
		'Permission to delete applications.'
	),
	('jobs:update', 'Permission to update jobs.'),
	('jobs:delete', 'Permission to delete jobs.'),
	(
		'deploy_key:read',
		'Permission to read deploy keys.'
	),
	(
		'deploy_key:update',
		'Permission to update deploy keys.'
	),
	(
		'unleash:create',
		'Permission to create unleash instances.'
	),
	(
		'unleash:update',
		'Permission to update unleash instances.'
	)
;

CREATE TABLE role_authorizations (
	role_name TEXT NOT NULL REFERENCES roles (name) ON DELETE CASCADE ON UPDATE CASCADE,
	authorization_name TEXT NOT NULL REFERENCES authorizations (name) ON DELETE CASCADE ON UPDATE CASCADE,
	PRIMARY KEY (role_name, authorization_name)
)
;

CREATE TABLE service_accounts (
	id UUID DEFAULT gen_random_uuid () PRIMARY KEY,
	created_at TIMESTAMP WITH TIME ZONE DEFAULT CLOCK_TIMESTAMP() NOT NULL,
	updated_at TIMESTAMP WITH TIME ZONE DEFAULT CLOCK_TIMESTAMP() NOT NULL,
	name TEXT NOT NULL CONSTRAINT name_length CHECK (CHAR_LENGTH(name) <= 80),
	description TEXT NOT NULL,
	team_slug slug REFERENCES teams (slug) ON DELETE CASCADE
)
;

INSERT INTO
	roles (name, description, is_only_global)
VALUES
	(
		'Deploy key viewer',
		'Permits the actor to view deploy keys.',
		FALSE
	),
	(
		'Service account owner',
		'Permits the actor to manage service accounts.',
		FALSE
	),
	(
		'Team creator',
		'Permits the actor to create teams.',
		TRUE
	),
	(
		'Team member',
		'Permits the actor to do actions on behalf of a team. Also includes managing most team resources except members.',
		FALSE
	),
	(
		'Team owner',
		'Permits the actor to do actions on behalf of a team. Also includes managing all team resources, including members.',
		FALSE
	)
;

INSERT INTO
	role_authorizations (role_name, authorization_name)
VALUES
	('Deploy key viewer', 'deploy_key:read'),
	('Deploy key viewer', 'deploy_key:update'),
	(
		'Service account owner',
		'service_accounts:create'
	),
	(
		'Service account owner',
		'service_accounts:delete'
	),
	('Service account owner', 'service_accounts:read'),
	(
		'Service account owner',
		'service_accounts:update'
	),
	('Team creator', 'teams:create'),
	('Team member', 'teams:metadata:update'),
	('Team member', 'deploy_key:read'),
	('Team member', 'jobs:delete'),
	('Team member', 'jobs:update'),
	('Team member', 'teams:secrets:create'),
	('Team member', 'teams:secrets:delete'),
	('Team member', 'teams:secrets:update'),
	('Team member', 'teams:secrets:read'),
	('Team member', 'teams:secrets:list'),
	('Team member', 'deploy_key:update'),
	('Team member', 'unleash:create'),
	('Team member', 'unleash:update'),
	('Team member', 'applications:update'),
	('Team member', 'applications:delete'),
	('Team member', 'repositories:create'),
	('Team member', 'repositories:delete'),
	('Team member', 'service_accounts:create'),
	('Team member', 'service_accounts:delete'),
	('Team member', 'service_accounts:read'),
	('Team member', 'service_accounts:update'),
	('Team owner', 'teams:delete'),
	('Team owner', 'teams:metadata:update'),
	('Team owner', 'teams:members:admin'),
	('Team owner', 'deploy_key:read'),
	('Team owner', 'jobs:delete'),
	('Team owner', 'jobs:update'),
	('Team owner', 'teams:secrets:create'),
	('Team owner', 'teams:secrets:delete'),
	('Team owner', 'teams:secrets:update'),
	('Team owner', 'teams:secrets:read'),
	('Team owner', 'teams:secrets:list'),
	('Team owner', 'deploy_key:update'),
	('Team owner', 'unleash:create'),
	('Team owner', 'unleash:update'),
	('Team owner', 'applications:update'),
	('Team owner', 'applications:delete'),
	('Team owner', 'repositories:create'),
	('Team owner', 'repositories:delete'),
	('Team owner', 'service_accounts:create'),
	('Team owner', 'service_accounts:delete'),
	('Team owner', 'service_accounts:read'),
	('Team owner', 'service_accounts:update')
;

CREATE UNIQUE INDEX ON service_accounts USING btree (name, team_slug) NULLS NOT DISTINCT
;

CREATE TRIGGER service_accounts_set_updated BEFORE
UPDATE ON service_accounts FOR EACH ROW
EXECUTE PROCEDURE set_updated_at ()
;

CREATE TABLE service_account_tokens (
	id UUID DEFAULT gen_random_uuid () PRIMARY KEY,
	created_at TIMESTAMP WITH TIME ZONE DEFAULT CLOCK_TIMESTAMP() NOT NULL,
	updated_at TIMESTAMP WITH TIME ZONE DEFAULT CLOCK_TIMESTAMP() NOT NULL,
	last_used_at TIMESTAMP WITH TIME ZONE,
	expires_at DATE,
	name TEXT NOT NULL CONSTRAINT name_length CHECK (CHAR_LENGTH(name) <= 80),
	description TEXT NOT NULL,
	token TEXT NOT NULL UNIQUE,
	service_account_id UUID NOT NULL REFERENCES service_accounts (id) ON DELETE CASCADE
)
;

CREATE UNIQUE INDEX ON service_account_tokens USING btree (service_account_id, name)
;

CREATE TRIGGER service_account_tokens_set_updated BEFORE
UPDATE ON service_account_tokens FOR EACH ROW
EXECUTE PROCEDURE set_updated_at ()
;

CREATE TABLE service_account_roles (
	id SERIAL PRIMARY KEY,
	created_at TIMESTAMP WITH TIME ZONE DEFAULT CLOCK_TIMESTAMP() NOT NULL,
	role_name TEXT NOT NULL REFERENCES roles (name) ON DELETE CASCADE ON UPDATE CASCADE,
	service_account_id UUID NOT NULL REFERENCES service_accounts (id) ON DELETE CASCADE
)
;

CREATE UNIQUE INDEX ON service_account_roles USING btree (service_account_id, role_name)
;

DELETE FROM user_roles
WHERE
	role_name IN (
		'Service account creator',
		'Admin',
		'User viewer',
		'Team viewer'
	)
;

ALTER TABLE user_roles
ALTER COLUMN role_name TYPE TEXT,
ADD CONSTRAINT user_roles_role_name_fkey FOREIGN KEY (role_name) REFERENCES roles (name) ON DELETE CASCADE ON UPDATE CASCADE
;

DROP TYPE role_name
;
