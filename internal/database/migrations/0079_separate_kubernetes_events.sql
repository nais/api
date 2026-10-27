-- +goose Up
DROP VIEW activity_log_combined_view
;

DROP MATERIALIZED VIEW activity_log_subset_mat_view
;

-- Kubernetes events are queried directly from k8s_events. Only deployments
-- remain in the materialized portion of the activity log.
CREATE MATERIALIZED VIEW activity_log_subset_mat_view AS
SELECT
	resources.id,
	resources.created_at,
	COALESCE(
		deployments.deployer_username,
		'Unknown GitHub user'
	) AS actor,
	'DEPLOYMENT'::TEXT AS action,
	CASE
		WHEN resources.kind = 'Naisjob' THEN 'JOB'
		ELSE 'APP'
	END AS resource_type,
	resources.name AS resource_name,
	deployments.team_slug,
	(
		JSONB_BUILD_OBJECT('triggerURL', deployments.trigger_url)::TEXT
	)::BYTEA AS data,
	deployments.environment_name AS environment
FROM
	deployment_k8s_resources AS resources
	JOIN deployments ON deployments.id = resources.deployment_id
WHERE
	resources.kind IN ('Application', 'Naisjob')
	AND resources.group = 'nais.io'
	AND resources.created_at >= NOW() - INTERVAL '6 months'
;

CREATE UNIQUE INDEX ON activity_log_subset_mat_view (id)
;

CREATE INDEX ON activity_log_subset_mat_view (created_at)
;

CREATE INDEX ON activity_log_subset_mat_view (team_slug)
;

CREATE INDEX ON activity_log_subset_mat_view (resource_type, resource_name, environment)
;

CREATE VIEW activity_log_combined_view AS
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

CREATE INDEX ON k8s_events (
	involved_namespace,
	environment_name,
	(data ->> 'workloadKind'),
	(data ->> 'workloadName'),
	triggered_at DESC,
	id DESC
)
;
