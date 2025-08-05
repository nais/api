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
	app_label,
	environment,
	DATE_TRUNC('month', date)::date AS MONTH,
	service,
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
	AND c.app_label = @app_label
	AND c.environment = @environment::TEXT
	AND c.date >= DATE_TRUNC('month', last_run) - INTERVAL '1 year'
GROUP BY
	team_slug,
	app_label,
	environment,
	service,
	MONTH
ORDER BY
	MONTH DESC
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

-- name: MonthlyCostForTenant :many
SELECT
	*
FROM
	cost_monthly_tenant
WHERE
	MONTH >= @from_date
	AND MONTH <= @to_date
ORDER BY
	MONTH ASC
;

-- name: CostUpsert :batchexec
INSERT INTO
	cost (
		environment,
		team_slug,
		app_label,
		service,
		date,
		daily_cost
	)
VALUES
	(
		@environment,
		@team_slug,
		@app_label,
		@service,
		@date,
		@daily_cost
	)
ON CONFLICT ON CONSTRAINT daily_cost_key DO UPDATE
SET
	daily_cost = EXCLUDED.daily_cost
;

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
	),
	cost_data AS (
		SELECT
			cost.date AS date,
			cost.environment AS environment,
			cost.team_slug AS team_slug,
			cost.app_label AS app_label,
			cost.service AS service,
			COALESCE(SUM(cost.daily_cost), 0)::REAL AS daily_cost
		FROM
			cost
		WHERE
			cost.date >= @from_date::date
			AND cost.date <= @to_date::date
			AND environment = @environment::TEXT
			AND team_slug = @team_slug::slug
			AND app_label = @app_label
		GROUP BY
			cost.date,
			cost.environment,
			cost.team_slug,
			cost.app_label,
			cost.service
	)
SELECT
	date_range.date::date AS date,
	cost_data.environment,
	cost_data.team_slug,
	cost_data.app_label,
	cost_data.service,
	cost_data.daily_cost
FROM
	date_range
	LEFT OUTER JOIN cost_data ON cost_data.date = date_range.date
ORDER BY
	date_range.date,
	cost_data.service ASC
;

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
	),
	cost_data AS (
		SELECT
			cost.date AS date,
			cost.service AS service,
			COALESCE(SUM(cost.daily_cost), 0)::REAL AS daily_cost
		FROM
			cost
		WHERE
			cost.date >= @from_date::date
			AND cost.date <= @to_date::date
			AND team_slug = @team_slug::slug
			AND CASE
				WHEN sqlc.narg(services)::TEXT[] IS NOT NULL THEN cost.service = ANY (@services)
				ELSE TRUE
			END
		GROUP BY
			cost.date,
			cost.service
	)
SELECT
	date_range.date::date AS date,
	cost_data.service,
	COALESCE(cost_data.daily_cost, 0) AS cost
FROM
	date_range
	LEFT OUTER JOIN cost_data ON cost_data.date = date_range.date
ORDER BY
	date_range.date,
	cost_data.service ASC
;

-- name: DailyEnvCostForTeam :many
SELECT
	team_slug,
	app_label,
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
	app_label,
	date
ORDER BY
	date,
	app_label ASC
;

-- name: CostForService :one
SELECT
	COALESCE(SUM(daily_cost), 0)::REAL
FROM
	cost
WHERE
	team_slug = @team_slug
	AND service = @service
	AND app_label = @app_label
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
	AND service = @service
	AND date >= @from_date
	AND date <= @to_date
;

-- name: RefreshCostMonthlyTeam :exec
REFRESH MATERIALIZED VIEW CONCURRENTLY cost_monthly_team
;

-- name: RefreshCostMonthlyTenant :exec
REFRESH MATERIALIZED VIEW CONCURRENTLY cost_monthly_tenant
;

-- name: DailyCostForTeamEnvironment :many
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
	),
	cost_data AS (
		SELECT
			cost.date AS date,
			cost.environment AS environment,
			cost.team_slug AS team_slug,
			cost.app_label AS app_label,
			COALESCE(SUM(cost.daily_cost), 0)::REAL AS daily_cost
		FROM
			cost
		WHERE
			cost.date >= @from_date::date
			AND cost.date <= @to_date::date
			AND environment = @environment::TEXT
			AND team_slug = @team_slug::slug
		GROUP BY
			cost.date,
			cost.environment,
			cost.team_slug,
			cost.app_label
	)
SELECT
	date_range.date::date AS date,
	cost_data.environment,
	cost_data.team_slug,
	cost_data.app_label,
	COALESCE(cost_data.daily_cost, 0) AS daily_cost
FROM
	date_range
	LEFT OUTER JOIN cost_data ON cost_data.date = date_range.date
ORDER BY
	date_range.date,
	cost_data.app_label ASC
;

-- name: ListTeamSlugsForCostUpdater :many
SELECT
	slug
FROM
	teams
ORDER BY
	teams.slug ASC
;
