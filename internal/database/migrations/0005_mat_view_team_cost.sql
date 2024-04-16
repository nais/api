-- +goose Up

-- Team range for resource utilization
DROP MATERIALIZED VIEW IF EXISTS cost_monthly_team;
CREATE MATERIALIZED VIEW cost_monthly_team AS
WITH last_run AS (
    SELECT MAX(date)::date AS "last_run"
    FROM cost
)
SELECT
    team_slug,
    date_trunc('month', date)::date AS month,
    -- Extract last day of known cost samples for the month, or the last recorded date
    -- This helps with estimation etc
    MAX(CASE
        WHEN date_trunc('month', date) < date_trunc('month', last_run) THEN date_trunc('month', date) + interval '1 month' - interval '1 day'
        ELSE date_trunc('day', last_run)
    END)::date AS last_recorded_date,
    SUM(daily_cost)::real AS daily_cost
FROM cost c
LEFT JOIN last_run ON true
GROUP BY team_slug, month
;

CREATE UNIQUE INDEX ON cost_monthly_team (team_slug, month);
