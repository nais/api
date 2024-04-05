-- LastCostDate will return the last date that has a cost.
-- name: LastCostDate :one
SELECT
    MAX(date)::date AS date
FROM
    cost;

-- name: MonthlyCostForApp :many
WITH last_run AS (
    SELECT MAX(date)::date AS "last_run"
    FROM cost
)
SELECT
    team_slug,
    app,
    environment,
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
WHERE c.team_slug = @team_slug::slug
AND c.app = @app
AND c.environment = @environment::text
GROUP BY team_slug, app, environment, month
ORDER BY month DESC
LIMIT 12;

-- name: MonthlyCostForTeam :many
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
WHERE c.team_slug = @team_slug::slug
GROUP BY team_slug, month
ORDER BY month DESC
LIMIT 12;

-- CostUpsert will insert or update a cost record. If there is a conflict on the daily_cost_key constrant, the
-- daily_cost column will be updated.
-- name: CostUpsert :batchexec
INSERT INTO cost (environment, team_slug, app, cost_type, date, daily_cost)
VALUES (@environment, @team_slug, @app, @cost_type, @date, @daily_cost)
ON CONFLICT ON CONSTRAINT daily_cost_key DO
    UPDATE SET daily_cost = EXCLUDED.daily_cost;

-- DailyCostForApp will fetch the daily cost for a specific team app in a specific environment, across all cost types
-- in a date range.
-- name: DailyCostForApp :many
SELECT
    *
FROM
    cost
WHERE
    date >= @from_date::date
    AND date <= @to_date::date
    AND environment = @environment::text
    AND team_slug = @team_slug::slug
    AND app = @app
ORDER BY
    date, cost_type ASC;

-- DailyCostForTeam will fetch the daily cost for a specific team across all apps and envs in a date range.
-- name: DailyCostForTeam :many
SELECT
    *
FROM
    cost
WHERE
    date >= @from_date::date
    AND date <= @to_date::date
    AND team_slug = @team_slug::slug
ORDER BY
    date, environment, app, cost_type ASC;

-- DailyEnvCostForTeam will fetch the daily cost for a specific team and environment across all apps in a date range.
-- name: DailyEnvCostForTeam :many
SELECT
    team_slug,
    app,
    date,
    SUM(daily_cost)::real AS daily_cost
FROM
    cost
WHERE
    date >= @from_date::date
    AND date <= @to_date::date
    AND environment = @environment
    AND team_slug = @team_slug::slug
GROUP BY
    team_slug, app, date
ORDER BY
    date, app ASC;

-- name: CostForSqlInstance :one
SELECT
    COALESCE(SUM(daily_cost), 0)::real
FROM
    cost
WHERE
    team_slug = @team_slug
    AND cost_type = 'Cloud SQL'
    AND app = @app_name
    AND date >= @from_date
    AND date <= @to_date
    AND environment = @environment::text;