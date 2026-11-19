-- +goose Up
-- Preserve audit identity, timestamps, actor, resource scope and the original
-- payload verbatim. Do not recast historical subresource operations as database
-- creation/deletion. The new decoder treats their payload as opaque history.
UPDATE activity_log_entries
SET
	data = CONVERT_TO(
		JSONB_BUILD_OBJECT(
			'originalAction',
			action,
			'originalData',
			CONVERT_FROM(data, 'UTF8')
		)::TEXT,
		'UTF8'
	),
	action = 'HISTORICAL'
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
