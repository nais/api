-- +goose Up
ALTER TABLE k8s_events
DROP CONSTRAINT k8s_events_pkey,
ADD COLUMN id UUID NOT NULL DEFAULT gen_random_uuid () PRIMARY KEY
;

CREATE UNIQUE INDEX ON k8s_events (uid, triggered_at)
;

DROP VIEW IF EXISTS activity_log_combined_view
;

DROP MATERIALIZED VIEW IF EXISTS activity_log_subset_mat_view
;

CREATE MATERIALIZED VIEW activity_log_subset_mat_view AS
WITH
	k8s_scale_events AS (
		SELECT
			id AS id,
			triggered_at AS created_at,
			'system' AS actor,
			'AUTOSCALE' AS action,
			'APP' AS resource_type,
			involved_name AS resource_name,
			involved_namespace AS team_slug,
			(data::TEXT)::bytea AS data,
			environment_name AS environment
		FROM
			k8s_events
		WHERE
			involved_kind = 'HorizontalPodAutoscaler'
	),
	deployments AS (
		SELECT
			deployment_k8s_resources.id AS id,
			deployment_k8s_resources.created_at AS created_at,
			COALESCE(
				deployments.deployer_username,
				'Unknown GitHub user'
			) AS actor,
			'DEPLOYMENT' AS action,
			CASE
				WHEN deployment_k8s_resources.kind = 'Naisjob' THEN 'JOB'
				ELSE 'APP'
			END AS resource_type,
			deployment_k8s_resources.name AS resource_name,
			deployments.team_slug AS team_slug,
			(
				JSONB_BUILD_OBJECT('triggerURL', deployments.trigger_url)::TEXT
			)::bytea AS data,
			deployments.environment_name AS environment
		FROM
			deployment_k8s_resources
			JOIN deployments ON deployments.id = deployment_k8s_resources.deployment_id
		WHERE
			deployment_k8s_resources.kind IN ('Application', 'Naisjob')
			AND deployment_k8s_resources.group = 'nais.io'
			-- Only include the last 6 months of deployments
			AND deployment_k8s_resources.created_at >= NOW() - INTERVAL '6 months'
	),
	full_set AS (
		SELECT
			*
		FROM
			k8s_scale_events
		UNION ALL
		SELECT
			*
		FROM
			deployments
	)
SELECT
	*
FROM
	full_set
ORDER BY
	created_at DESC
;

CREATE OR REPLACE VIEW activity_log_combined_view AS
SELECT
	id,
	created_at,
	actor,
	action,
	resource_type,
	resource_name,
	team_slug,
	data,
	environment
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
	team_slug,
	data,
	environment
FROM
	activity_log_subset_mat_view
;

CREATE UNIQUE INDEX ON activity_log_subset_mat_view (id)
;

CREATE INDEX ON activity_log_subset_mat_view (created_at)
;

CREATE INDEX ON activity_log_subset_mat_view (team_slug)
;

CREATE INDEX ON activity_log_subset_mat_view (resource_type, resource_name, environment)
;
