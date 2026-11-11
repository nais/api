-- +goose Up
ALTER TABLE cost
ADD COLUMN postgres_name TEXT
;

-- Keep Postgres costs distinct from same-named workloads and Aiven services.
-- An empty app label is the existing importer representation of a null app.
ALTER TABLE cost
ADD CONSTRAINT cost_postgres_identity CHECK (
	postgres_name IS NULL
	OR (
		postgres_name <> ''
		AND app_label = ''
	)
)
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
	service_name,
	postgres_name
)
;

CREATE INDEX cost_postgres_lookup_idx ON cost (team_slug, environment, postgres_name, date)
WHERE
	postgres_name IS NOT NULL
;
