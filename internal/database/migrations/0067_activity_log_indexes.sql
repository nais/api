-- +goose Up
-- Fix the combined view so both sides of the UNION ALL have identical types
-- for team_slug. The activity_log_entries table uses the "slug" domain (over
-- text) while the materialized view uses plain text. Casting the table side
-- to ::text makes them match, which allows PostgreSQL to push WHERE predicates
-- down into the individual scans instead of doing full sequential scans.
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

-- Composite indexes on activity_log_entries, replacing old single-column ones.
DROP INDEX IF EXISTS audit_events_team_slug_idx
;

CREATE INDEX activity_log_entries_team_slug_created_at_idx ON activity_log_entries (team_slug, created_at DESC)
;

DROP INDEX IF EXISTS audit_events_resource_type_idx
;

-- Column order: resource_type, resource_name first so that ListForResource
-- (which only filters these two) can use the leading prefix. ListForResourceTeamAndEnvironment
-- uses all four equality columns so their order among themselves doesn't matter.
CREATE INDEX activity_log_entries_resource_lookup_idx ON activity_log_entries (
	resource_type,
	resource_name,
	team_slug,
	environment,
	created_at DESC
)
;

-- Composite indexes on activity_log_subset_mat_view, replacing old ones.
DROP INDEX IF EXISTS activity_log_subset_mat_view_team_slug_idx
;

CREATE INDEX activity_log_subset_mat_view_team_created_at_idx ON activity_log_subset_mat_view (team_slug, created_at DESC)
;

DROP INDEX IF EXISTS activity_log_subset_mat_view_resource_type_resource_name_en_idx
;

CREATE INDEX activity_log_subset_mat_view_resource_lookup_idx ON activity_log_subset_mat_view (
	resource_type,
	resource_name,
	team_slug,
	environment,
	created_at DESC
)
;
