-- +goose Up
ALTER TABLE user_roles
DROP COLUMN target_service_account_id
;

DROP TABLE service_account_roles
;

DROP TABLE api_keys
;

DROP TABLE service_accounts
;

CREATE TABLE roles (name TEXT PRIMARY KEY, description TEXT NOT NULL)
;

CREATE TABLE authorizations (name TEXT PRIMARY KEY, description TEXT NOT NULL)
;

INSERT INTO
	authorizations (name, description)
VALUES
	('activity_logs:read', 'Some description'),
	('service_accounts:create', 'Some description'),
	('service_accounts:delete', 'Some description'),
	('service_accounts:read', 'Some description'),
	('service_accounts:update', 'Some description'),
	('teams:create', 'Some description'),
	('teams:delete', 'Some description'),
	('teams:list', 'Some description'),
	('teams:read', 'Some description'),
	('teams:metadata:update', 'Some description'),
	('teams:members:admin', 'Some description'),
	('teams:secrets:create', 'Some description'),
	('teams:secrets:delete', 'Some description'),
	('teams:secrets:update', 'Some description'),
	('teams:secrets:read', 'Some description'),
	('teams:secrets:list', 'Some description'),
	('repositories:create', 'Some description'),
	('repositories:delete', 'Some description'),
	('applications:update', 'Some description'),
	('applications:delete', 'Some description'),
	('jobs:update', 'Some description'),
	('jobs:delete', 'Some description'),
	('deploy_key:read', 'Some description'),
	('deploy_key:update', 'Some description'),
	('unleash:create', 'Some description'),
	('unleash:update', 'Some description')
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
	name TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	team_slug slug REFERENCES teams (slug) ON DELETE CASCADE
)
;

INSERT INTO
	roles (name, description)
VALUES
	('Admin', 'Some description'),
	('Deploy key viewer', 'Some description'),
	('Service account owner', 'Some description'),
	('Team creator', 'Some description'),
	('Team member', 'Some description'),
	('Team owner', 'Some description'),
	('Team viewer', 'Some description')
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
	('Team member', 'activity_logs:read'),
	('Team member', 'teams:read'),
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
	('Team owner', 'activity_logs:read'),
	('Team owner', 'teams:delete'),
	('Team owner', 'teams:read'),
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
	('Team owner', 'service_accounts:update'),
	('Team viewer', 'activity_logs:read'),
	('Team viewer', 'teams:list'),
	('Team viewer', 'teams:read')
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
	expires_at DATE,
	note TEXT NOT NULL,
	token TEXT NOT NULL UNIQUE,
	service_account_id UUID NOT NULL REFERENCES service_accounts (id) ON DELETE CASCADE
)
;

CREATE TRIGGER service_account_tokens_set_updated BEFORE
UPDATE ON service_account_tokens FOR EACH ROW
EXECUTE PROCEDURE set_updated_at ()
;

CREATE TABLE service_account_roles (
	id SERIAL PRIMARY KEY,
	created_at TIMESTAMP WITH TIME ZONE DEFAULT CLOCK_TIMESTAMP() NOT NULL,
	role_name TEXT NOT NULL REFERENCES roles (name) ON DELETE CASCADE ON UPDATE CASCADE,
	service_account_id UUID NOT NULL REFERENCES service_accounts (id) ON DELETE CASCADE,
	target_team_slug slug REFERENCES teams (slug) ON DELETE CASCADE
)
;

CREATE UNIQUE INDEX ON service_account_roles USING btree (service_account_id, role_name)
WHERE
	target_team_slug IS NULL
;

CREATE UNIQUE INDEX ON service_account_roles USING btree (service_account_id, role_name, target_team_slug)
WHERE
	target_team_slug IS NOT NULL
;

DELETE FROM user_roles
WHERE
	role_name = 'Service account creator'
;

ALTER TABLE user_roles
ALTER COLUMN role_name TYPE TEXT,
ADD CONSTRAINT user_roles_role_name_fkey FOREIGN KEY (role_name) REFERENCES roles (name) ON DELETE CASCADE ON UPDATE CASCADE
;

DROP TYPE role_name
;
