-- +goose Up
-- Delete historical elevation activity log entries
-- These are replaced by SECRET_VALUES_VIEWED entries in the new system
DELETE FROM activity_log_entries
WHERE
	resource_type = 'ELEVATION'
;

-- +goose Down
-- Cannot restore deleted data
