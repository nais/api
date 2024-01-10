-- +goose Up

-- types

CREATE DOMAIN slug AS
   TEXT CHECK (value ~ '^(?=.{3,30}$)[a-z](-?[a-z0-9]+)+$'::text);

CREATE TYPE reconciler_config_key AS ENUM (
    'azure:client_id',
    'azure:client_secret',
    'azure:tenant_id'
);

CREATE TYPE reconciler_name AS ENUM (
    'azure:group',
    'github:team',
    'google:gcp:gar',
    'google:gcp:project',
    'google:workspace-admin',
    'nais:dependencytrack',
    'nais:deploy',
    'nais:namespace'
);

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

CREATE TABLE first_run (
    first_run boolean NOT NULL
);

CREATE TABLE reconciler_errors (
    id BIGSERIAL,
    correlation_id uuid NOT NULL,
    reconciler reconciler_name NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    error_message text NOT NULL,
    team_slug slug NOT NULL,
    PRIMARY KEY(id),
    UNIQUE (team_slug, reconciler)
);

CREATE TABLE reconciler_config (
    reconciler reconciler_name NOT NULL,
    key reconciler_config_key NOT NULL,
    display_name text NOT NULL,
    description text NOT NULL,
    value text,
    secret boolean DEFAULT true NOT NULL,
    PRIMARY KEY (reconciler, key)
);

CREATE TABLE reconciler_states (
    reconciler reconciler_name NOT NULL,
    state jsonb DEFAULT '{}'::jsonb NOT NULL,
    team_slug slug NOT NULL,
    PRIMARY KEY (reconciler, team_slug)
);

CREATE TABLE reconciler_opt_outs (
    team_slug slug NOT NULL,
    user_id UUID NOT NULL,
    reconciler_name reconciler_name NOT NULL,
    PRIMARY KEY(team_slug, user_id, reconciler_name)
);

CREATE TABLE reconcilers (
    name reconciler_name NOT NULL,
    display_name text NOT NULL,
    description text NOT NULL,
    enabled boolean DEFAULT false NOT NULL,
    run_order integer NOT NULL,
    PRIMARY KEY(name),
    UNIQUE(display_name),
    UNIQUE(run_order)
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
    PRIMARY KEY(slug),
    CHECK ((TRIM(BOTH FROM purpose) <> ''::text)),
    CHECK ((slack_channel ~ '^#[a-z0-9æøå_-]{2,80}$'::text))
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
CREATE INDEX cost_env_idx ON cost (env);
CREATE INDEX cost_team_idx ON cost (team);
CREATE INDEX cost_app_idx ON cost (app);
CREATE INDEX cost_date_idx ON cost (date);
CREATE INDEX ON reconciler_errors USING btree (created_at DESC);
CREATE INDEX ON resource_utilization_metrics (app);
CREATE INDEX ON resource_utilization_metrics (env);
CREATE INDEX ON resource_utilization_metrics (resource_type);
CREATE INDEX ON resource_utilization_metrics (team);
CREATE INDEX ON resource_utilization_metrics (timestamp);
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

ALTER TABLE reconciler_states
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

ALTER TABLE user_roles
ADD FOREIGN KEY (target_service_account_id) REFERENCES service_accounts(id) ON DELETE CASCADE,
ADD FOREIGN KEY (target_team_slug) REFERENCES teams(slug) ON DELETE CASCADE,
ADD FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE resource_utilization_metrics
ADD FOREIGN KEY (team_slug) REFERENCES teams(slug) ON DELETE CASCADE;

-- data

INSERT INTO first_run VALUES(true);

INSERT INTO reconcilers
(name, display_name, description, enabled, run_order) VALUES
('github:team', 'GitHub teams', 'Create and maintain GitHub teams for the Console teams.', false, 1),
('azure:group', 'Azure AD groups', 'Create and maintain Azure AD security groups for the Console teams.', false, 2),
('google:workspace-admin', 'Google workspace group', 'Create and maintain Google workspace groups for the Console teams.', false, 3),
('google:gcp:project', 'GCP projects', 'Create GCP projects for the Console teams.', false, 4),
('nais:namespace', 'NAIS namespace', 'Create NAIS namespaces for the Console teams.', false, 5),
('nais:deploy', 'NAIS deploy', 'Provision NAIS deploy key for Console teams.', false, 6),
('google:gcp:gar', 'Google Artifact Registry', 'Provision artifact registry repositories for Console teams.', false, 7),
('nais:dependencytrack', 'DependencyTrack', 'Create teams and users in dependencytrack', false, 8);

INSERT INTO reconciler_config
(reconciler, key, display_name, description, secret) VALUES
('azure:group', 'azure:client_secret', 'Client secret', 'The client secret of the application registration.', true),
('azure:group', 'azure:client_id', 'Client ID', 'The client ID of the application registration that Console will use when communicating with the Azure AD APIs. The application must have the following API permissions: Group.Create, GroupMember.ReadWrite.All.', false),
('azure:group', 'azure:tenant_id', 'Tenant ID', 'The ID of the Azure AD tenant.', false);
