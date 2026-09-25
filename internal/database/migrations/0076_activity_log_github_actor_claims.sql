-- +goose Up
ALTER TABLE activity_log_entries
ADD COLUMN IF NOT EXISTS github_actor_claims JSONB
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
