-- +goose Up
UPDATE activity_log_entries
SET
	resource_type = 'VALKEY_INSTANCE',
	action = 'MAINTENANCE_STARTED'
WHERE
	resource_type = 'VALKEY_MAINTENANCE'
	AND action = 'STARTED'
;

UPDATE activity_log_entries
SET
	resource_type = 'OPENSEARCH',
	action = 'MAINTENANCE_STARTED'
WHERE
	resource_type = 'OPENSEARCH_MAINTENANCE'
	AND action = 'STARTED'
;
