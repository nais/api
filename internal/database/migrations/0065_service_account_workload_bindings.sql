-- +goose Up
CREATE TABLE service_account_workload_bindings (
	id UUID DEFAULT GEN_RANDOM_UUID() PRIMARY KEY,
	created_at TIMESTAMP WITH TIME ZONE DEFAULT CLOCK_TIMESTAMP() NOT NULL,
	updated_at TIMESTAMP WITH TIME ZONE DEFAULT CLOCK_TIMESTAMP() NOT NULL,
	last_used_at TIMESTAMP WITH TIME ZONE,
	service_account_id UUID NOT NULL REFERENCES service_accounts (id) ON DELETE CASCADE,
	environment TEXT NOT NULL,
	team_slug slug NOT NULL REFERENCES teams (slug) ON DELETE CASCADE,
	workload_name TEXT NOT NULL CONSTRAINT workload_name_length CHECK (CHAR_LENGTH(workload_name) <= 253),
	kubernetes_service_account_uid UUID
)
;

COMMENT ON COLUMN service_account_workload_bindings.kubernetes_service_account_uid IS 'The UID of the Kubernetes ServiceAccount, set on first successful authentication (trust-on-first-use).'
;

-- One workload (in a given env+team) can be bound to at most one Nais service account. Since a workload's
-- Kubernetes ServiceAccount lives in the team's namespace and there can only be one resource (Application or
-- Naisjob) by a given name there, the (environment, team_slug, workload_name) tuple is unique.
CREATE UNIQUE INDEX ON service_account_workload_bindings (environment, team_slug, workload_name)
;

CREATE INDEX ON service_account_workload_bindings (service_account_id)
;

CREATE TRIGGER service_account_workload_bindings_set_updated
BEFORE UPDATE ON service_account_workload_bindings FOR EACH ROW
EXECUTE PROCEDURE set_updated_at ()
;
