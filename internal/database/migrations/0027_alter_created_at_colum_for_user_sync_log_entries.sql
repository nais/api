-- +goose Up
ALTER TABLE usersync_log_entries
ALTER COLUMN created_at
SET DEFAULT CLOCK_TIMESTAMP()
;
