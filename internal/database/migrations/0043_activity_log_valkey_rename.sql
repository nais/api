-- +goose Up
UPDATE activity_log_entries
SET
	resource_type = 'VALKEY',
WHERE
	resource_type = 'VALKEY_INSTANCE'
;
