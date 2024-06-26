-- +goose Up

-- Grant permissions in GCP if the role cloudsqlsuperuser exists
-- +goose StatementBegin
DO
$$
BEGIN
   IF EXISTS (
      SELECT FROM pg_catalog.pg_roles
      WHERE  rolname = 'cloudsqlsuperuser') THEN
        GRANT ALL ON SCHEMA public TO cloudsqlsuperuser;
   END IF;
END
$$;
-- +goose StatementEnd

-- extensions

CREATE EXTENSION fuzzystrmatch;

-- functions

CREATE OR REPLACE FUNCTION set_updated_at() RETURNS TRIGGER AS
$$ BEGIN NEW.updated_at = NOW(); RETURN NEW; END; $$
    LANGUAGE plpgsql;

-- types

CREATE DOMAIN slug AS
   TEXT CHECK (value ~ '^(?=.{3,30}$)[a-z](-?[a-z0-9]+)+$'::text);

CREATE TYPE resource_type AS ENUM (
    'cpu',
    'memory'
);

CREATE TYPE role_name AS ENUM (
    'Admin',
    'Deploy key viewer',
    'Service account creator',
    'Service account owner',
    'Synchronizer',
    'Team creator',
    'Team member',
    'Team owner',
    'Team viewer',
    'User admin',
    'User viewer'
);

CREATE TYPE repository_authorization_enum AS ENUM (
    'deploy'
);

-- tables

CREATE TABLE api_keys (
    api_key text PRIMARY KEY,
    service_account_id uuid NOT NULL
);

CREATE TABLE audit_logs (
    id uuid DEFAULT gen_random_uuid() PRIMARY KEY,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    correlation_id uuid NOT NULL,
    component_name text NOT NULL,
    actor text,
    action text NOT NULL,
    message text NOT NULL,
    target_type text NOT NULL,
    target_identifier text NOT NULL
);


-- Cost_type is one of these
-- Cloud Key Management Service (KMS)
-- Compute Engine
-- InfluxDB
-- OpenSearch
-- Secret Manager
-- Cloud SQL
-- BigQuery
-- Kubernetes Engine
-- Redis
-- Cloud Storage
-- V and it should really, really be an enum.
CREATE TABLE cost (
    id serial PRIMARY KEY,
    environment text,
    team_slug slug,
    app text NOT NULL,
    cost_type text NOT NULL, --  some sort of string describing a cost center, maybe "redis"
    date date NOT NULL,
    daily_cost real NOT NULL,
    CONSTRAINT daily_cost_key UNIQUE (environment, team_slug, app, cost_type, date)
);

CREATE TABLE dependencytrack_projects(
    id uuid PRIMARY KEY,
    environment text NOT NULL,
    team_slug slug NOT NULL,
    app text NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE TABLE environments (
    name text PRIMARY KEY,
    gcp boolean DEFAULT false NOT NULL
);
COMMENT ON TABLE environments IS 'This table is used to store the environments that are available in the system. It will be emptied and repopulated when the system starts.';

CREATE TABLE reconciler_errors (
    id BIGSERIAL PRIMARY KEY,
    correlation_id uuid NOT NULL,
    reconciler text NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    error_message text NOT NULL,
    team_slug slug NOT NULL,
    UNIQUE (team_slug, reconciler)
);

CREATE TABLE reconciler_config (
    reconciler text NOT NULL,
    key text NOT NULL,
    display_name text NOT NULL,
    description text NOT NULL,
    value text,
    secret boolean DEFAULT true NOT NULL,
    PRIMARY KEY (reconciler, key)
);

CREATE TABLE reconciler_opt_outs (
    team_slug slug NOT NULL,
    user_id UUID NOT NULL,
    reconciler_name text NOT NULL,
    PRIMARY KEY (team_slug, user_id, reconciler_name)
);

CREATE TABLE reconciler_states (
    id UUID DEFAULT gen_random_uuid() NOT NULL PRIMARY KEY,
    reconciler_name text NOT NULL,
    team_slug slug NOT NULL,
    value bytea NOT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (reconciler_name, team_slug)
);

CREATE TABLE reconcilers (
    name text PRIMARY KEY,
    display_name text NOT NULL UNIQUE,
    description text NOT NULL,
    enabled boolean DEFAULT false NOT NULL,
    member_aware boolean DEFAULT false NOT NULL
);

CREATE TABLE repository_authorizations (
    team_slug slug NOT NULL,
    github_repository text NOT NULL,
    repository_authorization repository_authorization_enum NOT NULL,
    PRIMARY KEY (team_slug, github_repository, repository_authorization)
);

CREATE TABLE resource_utilization_metrics (
    id serial PRIMARY KEY,
    timestamp timestamp with time zone NOT NULL,
    environment text NOT NULL,
    team_slug slug NOT NULL,
    app text NOT NULL,
    resource_type resource_type NOT NULL,
    usage double precision NOT NULL,
    request double precision NOT NULL,
    CONSTRAINT resource_utilization_metric UNIQUE (timestamp, environment, team_slug, app, resource_type),
    CONSTRAINT positive_usage CHECK (usage > 0),
    CONSTRAINT positive_request CHECK (request > 0)
);

CREATE TABLE service_account_roles (
    id SERIAL PRIMARY KEY,
    role_name role_name NOT NULL,
    service_account_id uuid NOT NULL,
    target_team_slug slug,
    target_service_account_id uuid,
    CHECK (((target_team_slug IS NULL) OR (target_service_account_id IS NULL)))
);

CREATE TABLE service_accounts (
    id uuid DEFAULT gen_random_uuid() PRIMARY KEY,
    name text NOT NULL UNIQUE
);

CREATE TABLE sessions (
    id uuid DEFAULT gen_random_uuid() PRIMARY KEY,
    user_id uuid NOT NULL,
    expires timestamp with time zone NOT NULL
);

CREATE TABLE team_delete_keys (
    key uuid DEFAULT gen_random_uuid() PRIMARY KEY,
    team_slug slug NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    created_by uuid NOT NULL,
    confirmed_at timestamp with time zone
);

CREATE TABLE teams (
    slug slug PRIMARY KEY,
    purpose text NOT NULL,
    last_successful_sync timestamp without time zone,
    slack_channel text NOT NULL,
    google_group_email text,
    azure_group_id uuid,
    github_team_slug text,
    gar_repository text,
    CHECK ((TRIM(BOTH FROM purpose) <> ''::text)),
    CHECK ((slack_channel ~ '^#[a-z0-9æøå_-]{2,80}$'::text))
);

CREATE TABLE team_environments (
    id uuid DEFAULT gen_random_uuid() PRIMARY KEY,
    team_slug slug NOT NULL,
    environment text NOT NULL,
    slack_alerts_channel text,
    gcp_project_id text,
    UNIQUE (team_slug, environment),
    CHECK ((slack_alerts_channel IS NULL OR slack_alerts_channel ~ '^#[a-z0-9æøå_-]{2,80}$'::text))
);

CREATE TABLE user_roles (
    id SERIAL PRIMARY KEY,
    role_name role_name NOT NULL,
    user_id uuid NOT NULL,
    target_team_slug slug,
    target_service_account_id uuid,
    CHECK (((target_team_slug IS NULL) OR (target_service_account_id IS NULL)))
);

CREATE TABLE users (
    id uuid DEFAULT gen_random_uuid() PRIMARY KEY,
    email text NOT NULL UNIQUE,
    name text NOT NULL,
    external_id text NOT NULL UNIQUE
);

CREATE TABLE vulnerability_metrics(
    id uuid DEFAULT gen_random_uuid() PRIMARY KEY,
    date date NOT NULL,
    dependencytrack_project_id uuid NOT NULL,
    critical integer NOT NULL,
    high integer NOT NULL,
    medium integer NOT NULL,
    low integer NOT NULL,
    unassigned integer NOT NULL,
    risk_score double precision NOT NULL,
    CONSTRAINT vulnerability_metric UNIQUE (date, dependencytrack_project_id)
);

-- views

CREATE VIEW team_all_environments AS (
    SELECT
        teams.slug as team_slug,
        environments.name as environment,
        environments.gcp as gcp,
        team_environments.gcp_project_id,
        COALESCE(team_environments.id, gen_random_uuid())::uuid as id,
        COALESCE(team_environments.slack_alerts_channel, teams.slack_channel) as slack_alerts_channel
    FROM teams
    CROSS JOIN environments
    LEFT JOIN team_environments ON
        team_environments.team_slug = teams.slug
        AND team_environments.environment = environments.name

);

-- additional indexes

CREATE INDEX ON audit_logs USING btree (created_at DESC);

CREATE INDEX ON cost (environment);
CREATE INDEX ON cost (team_slug);
CREATE INDEX ON cost (app);
CREATE INDEX ON cost (date);

CREATE INDEX ON reconciler_errors USING btree (created_at DESC);
CREATE INDEX ON resource_utilization_metrics (app);
CREATE INDEX ON resource_utilization_metrics (environment);
CREATE INDEX ON resource_utilization_metrics (resource_type);
CREATE INDEX ON resource_utilization_metrics (team_slug);
CREATE INDEX ON resource_utilization_metrics (timestamp);

CREATE UNIQUE INDEX ON service_account_roles USING btree (service_account_id, role_name) WHERE ((target_team_slug IS NULL) AND (target_service_account_id IS NULL));
CREATE UNIQUE INDEX ON service_account_roles USING btree (service_account_id, role_name, target_service_account_id) WHERE (target_service_account_id IS NOT NULL);
CREATE UNIQUE INDEX ON service_account_roles USING btree (service_account_id, role_name, target_team_slug) WHERE (target_team_slug IS NOT NULL);

CREATE UNIQUE INDEX ON user_roles USING btree (user_id, role_name) WHERE ((target_team_slug IS NULL) AND (target_service_account_id IS NULL));
CREATE UNIQUE INDEX ON user_roles USING btree (user_id, role_name, target_service_account_id) WHERE (target_service_account_id IS NOT NULL);
CREATE UNIQUE INDEX ON user_roles USING btree (user_id, role_name, target_team_slug) WHERE (target_team_slug IS NOT NULL);

-- foreign keys

ALTER TABLE api_keys
    ADD FOREIGN KEY (service_account_id) REFERENCES service_accounts(id) ON DELETE CASCADE;

ALTER TABLE dependencytrack_projects
    ADD FOREIGN KEY (team_slug) REFERENCES teams(slug) ON DELETE CASCADE;

ALTER TABLE repository_authorizations
    ADD FOREIGN KEY (team_slug) REFERENCES teams(slug) ON DELETE CASCADE;

ALTER TABLE reconciler_config
    ADD FOREIGN KEY (reconciler) REFERENCES reconcilers(name) ON DELETE CASCADE;

ALTER TABLE reconciler_errors
    ADD FOREIGN KEY (reconciler) REFERENCES reconcilers(name) ON DELETE CASCADE,
    ADD FOREIGN KEY (team_slug) REFERENCES teams(slug) ON DELETE CASCADE;

ALTER TABLE reconciler_opt_outs
    ADD FOREIGN KEY (team_slug) REFERENCES teams(slug) ON DELETE CASCADE,
    ADD FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    ADD FOREIGN KEY (reconciler_name) REFERENCES reconcilers(name) ON DELETE CASCADE;

ALTER TABLE reconciler_states
    ADD FOREIGN KEY (reconciler_name) REFERENCES reconcilers(name) ON DELETE CASCADE,
    ADD FOREIGN KEY (team_slug) REFERENCES teams(slug) ON DELETE CASCADE;

ALTER TABLE resource_utilization_metrics
    ADD FOREIGN KEY (team_slug) REFERENCES teams(slug) ON DELETE CASCADE;

ALTER TABLE service_account_roles
    ADD FOREIGN KEY (service_account_id) REFERENCES service_accounts(id) ON DELETE CASCADE,
    ADD FOREIGN KEY (target_service_account_id) REFERENCES service_accounts(id) ON DELETE CASCADE,
    ADD FOREIGN KEY (target_team_slug) REFERENCES teams(slug) ON DELETE CASCADE;

ALTER TABLE sessions
    ADD FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE team_delete_keys
    ADD FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE CASCADE,
    ADD FOREIGN KEY (team_slug) REFERENCES teams(slug) ON DELETE CASCADE;

ALTER TABLE team_environments
    ADD FOREIGN KEY (team_slug) REFERENCES teams(slug) ON DELETE CASCADE;

ALTER TABLE user_roles
    ADD FOREIGN KEY (target_service_account_id) REFERENCES service_accounts(id) ON DELETE CASCADE,
    ADD FOREIGN KEY (target_team_slug) REFERENCES teams(slug) ON DELETE CASCADE,
    ADD FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE vulnerability_metrics
    ADD FOREIGN KEY (dependencytrack_project_id) REFERENCES dependencytrack_projects(id) ON DELETE CASCADE;

ALTER TABLE cost
    ADD FOREIGN KEY (team_slug) REFERENCES teams(slug) ON DELETE CASCADE;

-- triggers
CREATE TRIGGER reconciler_states_set_updated BEFORE
    UPDATE
    ON reconciler_states FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
