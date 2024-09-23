-- LastCostDate will return the last date that has a cost.
-- name: LastCostDate :one
SELECT
	MAX(date)::date AS date
FROM
	cost
;

-- name: MonthlyCostForWorkload :many
WITH
	last_run AS (
		SELECT
			MAX(date)::date AS "last_run"
		FROM
			cost
	)
SELECT
	team_slug,
	app AS workload,
	environment,
	DATE_TRUNC('month', date)::date AS MONTH,
	cost_type AS service,
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
WHERE
	c.team_slug = @team_slug::slug
	AND c.app = @workload
	AND c.environment = @environment::TEXT
GROUP BY
	team_slug,
	app,
	environment,
	service,
	MONTH
ORDER BY
	MONTH DESC
LIMIT
	12
;

-- name: MonthlyCostForTeam :many
SELECT
	*
FROM
	cost_monthly_team
WHERE
	team_slug = @team_slug::slug
ORDER BY
	MONTH DESC
LIMIT
	12
;

-- CostUpsert will insert or update a cost record. If there is a conflict on the daily_cost_key constrant, the
-- daily_cost column will be updated.
-- name: CostUpsert :batchexec
INSERT INTO
	cost (
		environment,
		team_slug,
		app,
		cost_type,
		date,
		daily_cost
	)
VALUES
	(
		@environment,
		@team_slug,
		@app,
		@cost_type,
		@date,
		@daily_cost
	)
ON CONFLICT ON CONSTRAINT daily_cost_key DO
UPDATE
SET
	daily_cost = EXCLUDED.daily_cost
;

-- DailyCostForWorkload will fetch the daily cost for a specific workload in an environment, across all cost types in a
-- date range.
-- name: DailyCostForWorkload :many
WITH
	date_range AS (
		SELECT
			*
		FROM
			GENERATE_SERIES(
				@from_date::date,
				@to_date::date,
				'1 day'::INTERVAL
			) AS date
	)
SELECT
	date_range.date::date AS date,
	cost.environment,
	cost.team_slug,
	cost.cost_type AS service,
	cost.daily_cost
FROM
	date_range
	LEFT OUTER JOIN cost ON cost.date = date_range.date
WHERE
	environment IS NULL
	OR (
		environment = @environment::TEXT
		AND team_slug = @team_slug::slug
		AND app = @workload
	)
ORDER BY
	date_range.date,
	service ASC
;

-- DailyCostForTeam will fetch the daily cost for a specific team across all apps and envs in a date range.
-- name: DailyCostForTeam :many
WITH
	date_range AS (
		SELECT
			*
		FROM
			GENERATE_SERIES(
				@from_date::date,
				@to_date::date,
				'1 day'::INTERVAL
			) AS date
	)
SELECT
	date_range.date::date AS date,
	cost.cost_type AS service,
	COALESCE(SUM(cost.daily_cost), 0)::REAL AS cost
FROM
	date_range
	LEFT OUTER JOIN cost ON cost.date = date_range.date
WHERE
	team_slug IS NULL
	OR team_slug = @team_slug::slug
GROUP BY
	date_range.date,
	service
ORDER BY
	date_range.date,
	service ASC
;

-- DailyEnvCostForTeam will fetch the daily cost for a specific team and environment across all apps in a date range.
-- name: DailyEnvCostForTeam :many
SELECT
	team_slug,
	app AS workload,
	date,
	SUM(daily_cost)::REAL AS daily_cost
FROM
	cost
WHERE
	date >= @from_date::date
	AND date <= @to_date::date
	AND environment = @environment
	AND team_slug = @team_slug::slug
GROUP BY
	team_slug,
	app,
	date
ORDER BY
	date,
	app ASC
;

-- name: CostForInstance :one
SELECT
	COALESCE(SUM(daily_cost), 0)::REAL
FROM
	cost
WHERE
	team_slug = @team_slug
	AND cost_type = @cost_type
	AND app = @workload
	AND date >= @from_date
	AND date <= @to_date
	AND environment = @environment::TEXT
;

-- name: CostForTeam :one
SELECT
	COALESCE(SUM(daily_cost), 0)::REAL
FROM
	cost
WHERE
	team_slug = @team_slug
	AND cost_type = @cost_type
	AND date >= @from_date
	AND date <= @to_date
;

-- name: CurrentSqlInstancesCostForTeam :one
SELECT
	COALESCE(SUM(daily_cost), 0)::REAL
FROM
	cost
WHERE
	team_slug = @team_slug
	AND cost_type = 'Cloud SQL'
	AND date >= @from_date
	AND date <= @to_date
;

-- name: RefreshCostMonthlyTeam :exec
REFRESH MATERIALIZED VIEW CONCURRENTLY cost_monthly_team
;
