-- +goose Up
DROP VIEW activity_log_combined_view
;

-- Audit namespaces can exceed the team slug domain's 30-character limit.
ALTER TABLE activity_log_entries
ALTER COLUMN team_slug TYPE TEXT USING team_slug::TEXT,
ADD COLUMN kubernetes_audit_uid UUID
;

CREATE VIEW activity_log_combined_view AS
SELECT
	id,
	created_at,
	actor,
	action,
	resource_type,
	resource_name,
	team_slug,
	data,
	environment,
	github_actor_claims
FROM
	activity_log_entries
UNION ALL
SELECT
	id,
	created_at,
	actor,
	action,
	resource_type,
	resource_name,
	team_slug::TEXT AS team_slug,
	data,
	environment,
	NULL::JSONB AS github_actor_claims
FROM
	activity_log_subset_mat_view
;

-- Preserve the deduplication previously provided by k8s_events.
CREATE UNIQUE INDEX ON activity_log_entries (kubernetes_audit_uid, created_at)
WHERE
	kubernetes_audit_uid IS NOT NULL
;

WITH
	audit_events AS (
		DELETE FROM k8s_events
		WHERE
			data ->> 'user' IS NOT NULL
		RETURNING
			*
	)
INSERT INTO
	activity_log_entries (
		id,
		created_at,
		actor,
		action,
		resource_type,
		resource_name,
		team_slug,
		data,
		environment,
		kubernetes_audit_uid
	)
SELECT
	id,
	triggered_at,
	data ->> 'user',
	'CLUSTER_AUDIT',
	'CLUSTER_AUDIT',
	involved_name,
	involved_namespace,
	CONVERT_TO(
		JSONB_BUILD_OBJECT('action', reason, 'resourceKind', involved_kind)::TEXT,
		'UTF8'
	),
	environment_name,
	uid
FROM
	audit_events
ON CONFLICT (kubernetes_audit_uid, created_at)
WHERE
	kubernetes_audit_uid IS NOT NULL DO NOTHING
;
