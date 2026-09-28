-- +goose Up
ALTER TABLE activity_log_entries
ADD COLUMN IF NOT EXISTS github_actor_claims JSONB
;

-- Move legacy claims out of data now that all writers use the dedicated column.
-- Safe to rerun: only rows still containing the legacy key are updated.
WITH
	entries AS (
		SELECT
			id,
			CONVERT_FROM(data, 'UTF8')::JSONB AS payload
		FROM
			activity_log_entries
		WHERE
			data IS NOT NULL
	)
UPDATE activity_log_entries AS entry
SET
	github_actor_claims = COALESCE(
		entry.github_actor_claims,
		NULLIF(
			entries.payload -> 'gitHubActorClaims',
			'null'::JSONB
		)
	),
	data = CONVERT_TO(
		(entries.payload - 'gitHubActorClaims')::TEXT,
		'UTF8'
	)
FROM
	entries
WHERE
	entry.id = entries.id
	AND JSONB_TYPEOF(entries.payload) = 'object'
	AND entries.payload ? 'gitHubActorClaims'
;

CREATE OR REPLACE VIEW activity_log_combined_view AS
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
	team_slug,
	data,
	environment,
	NULL::JSONB AS github_actor_claims
FROM
	activity_log_subset_mat_view
;
