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

CREATE TABLE service_accounts (
	id UUID DEFAULT gen_random_uuid () PRIMARY KEY,
	created_at TIMESTAMP WITH TIME ZONE DEFAULT CLOCK_TIMESTAMP() NOT NULL,
	updated_at TIMESTAMP WITH TIME ZONE DEFAULT CLOCK_TIMESTAMP() NOT NULL,
	name TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	team_slug slug REFERENCES teams (slug) ON DELETE CASCADE
)
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
	role_name role_name NOT NULL,
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
