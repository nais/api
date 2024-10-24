-- +goose Up
-- Team range for resource utilization
DROP MATERIALIZED VIEW IF EXISTS cost_monthly_team
;

CREATE MATERIALIZED VIEW cost_monthly_team AS
WITH
	last_run AS (
		SELECT
			MAX(date)::date AS "last_run"
		FROM
			cost
	)
SELECT
	team_slug,
	DATE_TRUNC('month', date)::date AS MONTH,
	-- Extract last day of known cost samples for the month, or the last recorded date
	-- This helps with estimation etc
	MAX(
		CASE
			WHEN DATE_TRUNC('month', date) < DATE_TRUNC('month', last_run) THEN DATE_TRUNC('month', date) + INTERVAL '1 month' - INTERVAL '1 day'
			ELSE DATE_TRUNC('day', last_run)
		END
	)::date AS last_recorded_date,
	SUM(daily_cost)::REAL AS daily_cost
FROM
	cost c
	LEFT JOIN last_run ON TRUE
GROUP BY
	team_slug,
	MONTH
;

CREATE UNIQUE INDEX ON cost_monthly_team (team_slug, MONTH)
;
