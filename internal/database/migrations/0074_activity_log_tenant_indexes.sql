-- +goose NO TRANSACTION
-- +goose Up
DROP INDEX CONCURRENTLY IF EXISTS activity_log_entries_team_scope_idx
;

CREATE INDEX CONCURRENTLY activity_log_entries_team_scope_idx ON activity_log_entries (team_slug, created_at DESC) INCLUDE (resource_type, action, environment)
;

DROP INDEX CONCURRENTLY IF EXISTS activity_log_subset_mat_view_team_scope_idx
;

CREATE INDEX CONCURRENTLY activity_log_subset_mat_view_team_scope_idx ON activity_log_subset_mat_view (team_slug, created_at DESC) INCLUDE (resource_type, action, environment)
;

DROP INDEX CONCURRENTLY IF EXISTS activity_log_entries_resource_scope_idx
;

CREATE INDEX CONCURRENTLY activity_log_entries_resource_scope_idx ON activity_log_entries (resource_type, resource_name, created_at DESC) INCLUDE (team_slug, environment, action)
;

DROP INDEX CONCURRENTLY IF EXISTS activity_log_subset_mat_view_resource_scope_idx
;

CREATE INDEX CONCURRENTLY activity_log_subset_mat_view_resource_scope_idx ON activity_log_subset_mat_view (resource_type, resource_name, created_at DESC) INCLUDE (team_slug, environment, action)
;

DROP INDEX CONCURRENTLY IF EXISTS activity_log_entries_team_slug_created_at_idx
;

DROP INDEX CONCURRENTLY IF EXISTS activity_log_subset_mat_view_team_created_at_idx
;

-- +goose Down
CREATE INDEX CONCURRENTLY IF NOT EXISTS activity_log_entries_team_slug_created_at_idx ON activity_log_entries (team_slug, created_at DESC)
;

CREATE INDEX CONCURRENTLY IF NOT EXISTS activity_log_subset_mat_view_team_created_at_idx ON activity_log_subset_mat_view (team_slug, created_at DESC)
;

DROP INDEX CONCURRENTLY IF EXISTS activity_log_entries_team_scope_idx
;

DROP INDEX CONCURRENTLY IF EXISTS activity_log_subset_mat_view_team_scope_idx
;

DROP INDEX CONCURRENTLY IF EXISTS activity_log_entries_resource_scope_idx
;

DROP INDEX CONCURRENTLY IF EXISTS activity_log_subset_mat_view_resource_scope_idx
;
