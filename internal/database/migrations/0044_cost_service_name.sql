-- +goose Up
ALTER TABLE cost
ADD COLUMN service_name TEXT
;

CREATE INDEX idx_cost_service_name ON cost (service_name)
;

-- daily_cost_key" UNIQUE CONSTRAINT, btree (environment, team_slug, app_label, service, date)
ALTER TABLE cost
DROP CONSTRAINT daily_cost_key
;

ALTER TABLE cost
ADD CONSTRAINT daily_cost_key UNIQUE (
	environment,
	team_slug,
	app_label,
	service,
	date,
	service_name
)
;
