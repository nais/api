-- +goose NO TRANSACTION
-- +goose Up
CREATE INDEX IF NOT EXISTS deployments_team_slug_created_at_idx ON deployments (team_slug, created_at DESC)
;

CREATE INDEX IF NOT EXISTS deployments_team_slug_environment_name_created_at_idx ON deployments (team_slug, environment_name, created_at DESC)
;

CREATE INDEX IF NOT EXISTS deployment_k8s_resources_deployment_id_created_at_idx ON deployment_k8s_resources (deployment_id, created_at DESC)
;

CREATE INDEX IF NOT EXISTS deployment_statuses_deployment_id_created_at_idx ON deployment_statuses (deployment_id, created_at DESC)
;

DROP INDEX CONCURRENTLY IF EXISTS deployments_created_at_idx
;

DROP INDEX CONCURRENTLY IF EXISTS deployments_team_slug_environment_name_idx
;

DROP INDEX CONCURRENTLY IF EXISTS deployment_k8s_resources_created_at_idx
;

DROP INDEX CONCURRENTLY IF EXISTS deployment_k8s_resources_deployment_id_idx
;

DROP INDEX CONCURRENTLY IF EXISTS deployment_statuses_created_at_idx
;

DROP INDEX CONCURRENTLY IF EXISTS deployment_statuses_deployment_id_idx
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
