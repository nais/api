-- +goose Up
-- Remove retired branch events, including the legacy branch-deletion action.
-- Current database creation, updates, access and deletion requests are retained.
DELETE FROM activity_log_entries
WHERE
	resource_type = 'POSTGRES'
	AND action IN (
		'BRANCH_CREATED',
		'BRANCH_ACTIVATED',
		'BRANCH_DELETED',
		'DELETED'
	)
;

-- Issues are derived state. The checker recreates current Postgres issues using
-- the logical database identity; obsolete physical-resource issues must not leak.
DELETE FROM issues
WHERE
	issue_type = 'POSTGRES_BRANCH'
	OR resource_type = 'POSTGRES_BRANCH'
;
