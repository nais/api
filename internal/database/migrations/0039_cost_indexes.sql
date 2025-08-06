-- +goose NO TRANSACTION
-- +goose Up
CREATE INDEX cost_team_app_env_date_idx ON cost (
	team_slug,
	date,
	environment,
	app_label,
	daily_cost
)
;

CREATE INDEX cost_team_service_date_idx ON cost (team_slug, date, service, daily_cost)
;

VACUUM
ANALYZE cost
;
