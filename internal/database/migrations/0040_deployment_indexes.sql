-- +goose NO TRANSACTION
-- +goose Up
CREATE INDEX ON deployments (team_slug, created_at DESC)
;

CREATE INDEX ON deployments (team_slug, environment_name, created_at DESC)
;

CREATE INDEX ON deployment_k8s_resources (deployment_id, created_at DESC)
;

CREATE INDEX ON deployment_statuses (deployment_id, created_at DESC)
;

DROP INDEX IF EXISTS deployments_created_at_idx
;

DROP INDEX IF EXISTS deployments_team_slug_environment_name_idx
;

DROP INDEX IF EXISTS deployment_k8s_resources_created_at_idx
;

DROP INDEX IF EXISTS deployment_k8s_resources_deployment_id_idx
;

DROP INDEX IF EXISTS deployment_statuses_created_at_idx
;

DROP INDEX IF EXISTS deployment_statuses_deployment_id_idx
;

VACUUM
ANALYZE deployments
;

VACUUM
ANALYZE deployment_k8s_resources
;

VACUUM
ANALYZE deployment_statuses
;
