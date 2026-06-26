-- +goose Up
-- Add created_at index for tenant-level activity log queries that filter by time range
-- without a leading team_slug or resource_type predicate.
CREATE INDEX activity_log_entries_created_at_idx ON activity_log_entries (created_at DESC)
;

CREATE INDEX activity_log_subset_mat_view_created_at_idx ON activity_log_subset_mat_view (created_at DESC)
;

-- +goose Down
DROP INDEX IF EXISTS activity_log_entries_created_at_idx
;

DROP INDEX IF EXISTS activity_log_subset_mat_view_created_at_idx
;
