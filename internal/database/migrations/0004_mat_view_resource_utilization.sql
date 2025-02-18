-- +goose Up
-- Team range for resource utilization
DROP MATERIALIZED VIEW IF EXISTS resource_team_range
;

CREATE MATERIALIZED VIEW resource_team_range AS
SELECT
	team_slug,
	MIN(TIMESTAMP)::TIMESTAMPTZ AS "from",
	MAX(TIMESTAMP)::TIMESTAMPTZ AS "to"
FROM
	resource_utilization_metrics
GROUP BY
	team_slug
;

CREATE UNIQUE INDEX ON resource_team_range (team_slug)
;
