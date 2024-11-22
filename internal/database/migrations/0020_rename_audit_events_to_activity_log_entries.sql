-- +goose Up
ALTER TABLE audit_events
RENAME TO activity_log_entries
;
