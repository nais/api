-- +goose Up

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
    api_key text NOT NULL,
    service_account_id uuid NOT NULL,
    PRIMARY KEY(api_key)
);

CREATE TABLE audit_logs (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    correlation_id uuid NOT NULL,
    component_name text NOT NULL,
    actor text,
    action text NOT NULL,
    message text NOT NULL,
    target_type text NOT NULL,
    target_identifier text NOT NULL,
    PRIMARY KEY(id)
);

CREATE TABLE cost (
    id serial PRIMARY KEY,
    environment text,
    team_slug slug,
    app text NOT NULL,
    cost_type text NOT NULL,
    date date NOT NULL,
    daily_cost real NOT NULL,
    CONSTRAINT daily_cost_key UNIQUE (environment, team_slug, app, cost_type, date)
);

CREATE TABLE reconciler_errors (
    id BIGSERIAL,
    correlation_id uuid NOT NULL,
    reconciler text NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    error_message text NOT NULL,
    team_slug slug NOT NULL,
    PRIMARY KEY(id),
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
    PRIMARY KEY(team_slug, user_id, reconciler_name)
);

CREATE TABLE reconciler_resources (
  id UUID DEFAULT gen_random_uuid() NOT NULL PRIMARY KEY,
  reconciler_name text NOT NULL,
  team_slug slug NOT NULL,
  name TEXT NOT NULL,
  value TEXT NOT NULL,
  metadata JSONB DEFAULT '{}'::jsonb NOT NULL,
  created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(reconciler_name, team_slug, name)
);

CREATE TABLE reconcilers (
    name text NOT NULL,
    display_name text NOT NULL,
    description text NOT NULL,
    enabled boolean DEFAULT false NOT NULL,
    member_aware boolean DEFAULT false NOT NULL,
    PRIMARY KEY(name),
    UNIQUE(display_name)
);

CREATE TABLE repository_authorizations (
    team_slug slug NOT NULL,
    github_repository text NOT NULL,
    repository_authorization repository_authorization_enum NOT NULL,
    PRIMARY KEY(team_slug, github_repository, repository_authorization)
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
    id SERIAL,
    role_name role_name NOT NULL,
    service_account_id uuid NOT NULL,
    target_team_slug slug,
    target_service_account_id uuid,
    PRIMARY KEY(id),
    CHECK (((target_team_slug IS NULL) OR (target_service_account_id IS NULL)))
);

CREATE TABLE service_accounts (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    name text NOT NULL,
    PRIMARY KEY(id),
    UNIQUE(name)
);

CREATE TABLE sessions (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    expires timestamp with time zone NOT NULL,
    PRIMARY KEY(id)
);

CREATE TABLE slack_alerts_channels (
    team_slug slug NOT NULL,
    environment text NOT NULL,
    channel_name text NOT NULL,
    PRIMARY KEY (team_slug, environment),
    CHECK ((channel_name ~ '^#[a-z0-9æøå_-]{2,80}$'::text))
);

CREATE TABLE team_delete_keys (
    key uuid DEFAULT gen_random_uuid() NOT NULL,
    team_slug slug NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    created_by uuid NOT NULL,
    confirmed_at timestamp with time zone,
    PRIMARY KEY(key)
);

CREATE TABLE teams (
    slug slug NOT NULL,
    purpose text NOT NULL,
    last_successful_sync timestamp without time zone,
    slack_channel text NOT NULL,
    google_group_email text,
    azure_group_id uuid,
    github_team_slug text,
    PRIMARY KEY(slug),
    CHECK ((TRIM(BOTH FROM purpose) <> ''::text)),
    CHECK ((slack_channel ~ '^#[a-z0-9æøå_-]{2,80}$'::text))
);

CREATE TABLE team_environments (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    team_slug slug NOT NULL,
    environment text NOT NULL,
    namespace text,
    gcp_project_id text,
    UNIQUE(team_slug, environment)
);

CREATE TABLE user_roles (
    id SERIAL,
    role_name role_name NOT NULL,
    user_id uuid NOT NULL,
    target_team_slug slug,
    target_service_account_id uuid,
    PRIMARY KEY(id),
    CHECK (((target_team_slug IS NULL) OR (target_service_account_id IS NULL)))
);

CREATE TABLE users (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    email text NOT NULL,
    name text NOT NULL,
    external_id text NOT NULL,
    PRIMARY KEY(id),
    UNIQUE(email),
    UNIQUE(external_id)
);

-- additional indexes

CREATE INDEX ON audit_logs USING btree (created_at DESC);
CREATE INDEX cost_env_idx ON cost (environment);
CREATE INDEX cost_team_idx ON cost (team_slug);
CREATE INDEX cost_app_idx ON cost (app);
CREATE INDEX cost_date_idx ON cost (date);
CREATE INDEX ON reconciler_errors USING btree (created_at DESC);
CREATE INDEX ON resource_utilization_metrics (app);
CREATE INDEX ON resource_utilization_metrics (environment);
CREATE INDEX ON resource_utilization_metrics (resource_type);
CREATE INDEX ON resource_utilization_metrics (team_slug);
CREATE INDEX ON resource_utilization_metrics (timestamp);
CREATE INDEX ON reconciler_resources (reconciler_name, name, team_slug);
CREATE UNIQUE INDEX ON service_account_roles USING btree (service_account_id, role_name) WHERE ((target_team_slug IS NULL) AND (target_service_account_id IS NULL));
CREATE UNIQUE INDEX ON user_roles USING btree (user_id, role_name) WHERE ((target_team_slug IS NULL) AND (target_service_account_id IS NULL));
CREATE UNIQUE INDEX ON service_account_roles USING btree (service_account_id, role_name, target_service_account_id) WHERE (target_service_account_id IS NOT NULL);
CREATE UNIQUE INDEX ON user_roles USING btree (user_id, role_name, target_service_account_id) WHERE (target_service_account_id IS NOT NULL);
CREATE UNIQUE INDEX ON service_account_roles USING btree (service_account_id, role_name, target_team_slug) WHERE (target_team_slug IS NOT NULL);
CREATE UNIQUE INDEX ON user_roles USING btree (user_id, role_name, target_team_slug) WHERE (target_team_slug IS NOT NULL);

-- foreign keys

ALTER TABLE api_keys
ADD FOREIGN KEY (service_account_id) REFERENCES service_accounts(id) ON DELETE CASCADE;

ALTER TABLE reconciler_config
ADD FOREIGN KEY (reconciler) REFERENCES reconcilers(name) ON DELETE CASCADE;

ALTER TABLE reconciler_errors
ADD FOREIGN KEY (reconciler) REFERENCES reconcilers(name) ON DELETE CASCADE,
ADD FOREIGN KEY (team_slug) REFERENCES teams(slug) ON DELETE CASCADE;

ALTER TABLE reconciler_opt_outs
ADD FOREIGN KEY (team_slug) REFERENCES teams(slug) ON DELETE CASCADE,
ADD FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
ADD FOREIGN KEY (reconciler_name) REFERENCES reconcilers(name) ON DELETE CASCADE;

ALTER TABLE repository_authorizations
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

ALTER TABLE resource_utilization_metrics
ADD FOREIGN KEY (team_slug) REFERENCES teams(slug) ON DELETE CASCADE;

ALTER TABLE reconciler_resources
ADD FOREIGN KEY (team_slug) REFERENCES teams(slug) ON DELETE CASCADE,
ADD FOREIGN KEY (reconciler_name) REFERENCES reconcilers(name) ON DELETE CASCADE;

-- triggers
CREATE TRIGGER reconciler_resources_set_updated
    BEFORE UPDATE
    ON reconciler_resources
    FOR EACH ROW
    EXECUTE PROCEDURE set_updated_at();
