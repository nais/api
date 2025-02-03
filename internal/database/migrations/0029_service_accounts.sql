-- +goose Up
DROP TABLE service_accounts
;

CREATE TABLE service_accounts (
	id UUID DEFAULT gen_random_uuid () PRIMARY KEY,
	created_at TIMESTAMP WITH TIME ZONE DEFAULT CLOCK_TIMESTAMP() NOT NULL,
	name TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	team_slug slug REFERENCES teams (slug) ON DELETE CASCADE
)
;

CREATE UNIQUE INDEX ON service_accounts USING btree (name, team_slug) NULLS NOT DISTINCT
;

DROP TABLE api_keys
;

CREATE TABLE service_account_tokens (
	id UUID DEFAULT gen_random_uuid () PRIMARY KEY,
	created_at TIMESTAMP WITH TIME ZONE DEFAULT CLOCK_TIMESTAMP() NOT NULL,
	expires_at DATE,
	note TEXT NOT NULL,
	token TEXT NOT NULL UNIQUE,
	service_account_id UUID NOT NULL REFERENCES teams (slug) ON DELETE CASCADE
)
;
