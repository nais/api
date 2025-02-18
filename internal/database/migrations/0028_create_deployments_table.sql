-- +goose Up
CREATE TYPE deployment_state AS ENUM(
	'success',
	'error',
	'failure',
	'inactive',
	'in_progress',
	'queued',
	'pending'
)
;

CREATE TABLE deployments (
	id UUID DEFAULT gen_random_uuid () PRIMARY KEY,
	external_id TEXT UNIQUE,
	created_at TIMESTAMP WITH TIME ZONE DEFAULT CLOCK_TIMESTAMP() NOT NULL,
	team_slug slug NOT NULL,
	repository TEXT,
	commit_sha TEXT,
	deployer_username TEXT,
	trigger_url TEXT,
	environment_name TEXT NOT NULL
)
;

CREATE TABLE deployment_k8s_resources (
	id UUID DEFAULT gen_random_uuid () PRIMARY KEY,
	created_at TIMESTAMP WITH TIME ZONE DEFAULT CLOCK_TIMESTAMP() NOT NULL,
	deployment_id UUID NOT NULL,
	"group" TEXT NOT NULL,
	version TEXT NOT NULL,
	kind TEXT NOT NULL,
	name TEXT NOT NULL,
	namespace TEXT NOT NULL
)
;

CREATE TABLE deployment_statuses (
	id UUID DEFAULT gen_random_uuid () PRIMARY KEY,
	created_at TIMESTAMP WITH TIME ZONE DEFAULT CLOCK_TIMESTAMP() NOT NULL,
	deployment_id UUID NOT NULL,
	state deployment_state NOT NULL,
	message TEXT NOT NULL
)
;

ALTER TABLE deployment_k8s_resources
ADD FOREIGN KEY (deployment_id) REFERENCES deployments (id) ON DELETE CASCADE
;

ALTER TABLE deployment_statuses
ADD FOREIGN KEY (deployment_id) REFERENCES deployments (id) ON DELETE CASCADE
;

CREATE INDEX ON deployments USING btree (created_at DESC)
;

CREATE INDEX ON deployments USING btree (team_slug, environment_name)
;

CREATE INDEX ON deployment_statuses USING btree (created_at DESC)
;

CREATE INDEX ON deployment_statuses USING btree (deployment_id)
;

CREATE INDEX ON deployment_k8s_resources USING btree (created_at DESC)
;

CREATE INDEX ON deployment_k8s_resources USING btree (deployment_id)
;

CREATE INDEX ON deployment_k8s_resources USING btree (kind, name)
;
