-- +goose Up
TRUNCATE TABLE cost
;

ALTER TABLE cost
DROP CONSTRAINT daily_cost_key
;

ALTER TABLE cost
ADD CONSTRAINT daily_cost_key UNIQUE NULLS NOT DISTINCT (
	environment,
	team_slug,
	app_label,
	service,
	date,
	service_name
)
;
