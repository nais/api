-- LastCostDate will return the last date that has a cost.
-- name: LastCostDate :one
SELECT
    MAX(date)::date AS date
FROM
    cost;

-- name: MonthlyCostForApp :many
SELECT *
FROM cost_monthly_app
WHERE team_slug = @team_slug::slug
AND app = @app
AND environment = @environment::text
ORDER BY month DESC
LIMIT 12;

-- name: MonthlyCostForTeam :many
SELECT *
FROM cost_monthly_team
WHERE team_slug = @team_slug::slug
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
SELECT *
FROM cost_daily_team
WHERE
    date >= @from_date::date
    AND date <= @to_date::date
    AND team_slug = @team_slug::slug
    AND environment = @environment
ORDER BY
    date, app ASC;
