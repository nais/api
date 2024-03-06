-- +goose Up

-- Team range for resource utilization
CREATE MATERIALIZED VIEW resource_team_range AS
SELECT
  team_slug,
  MIN(timestamp)::timestamptz AS "from",
  MAX(timestamp)::timestamptz AS "to"
FROM
  resource_utilization_metrics
GROUP BY
  team_slug;

CREATE INDEX ON resource_team_range (team_slug);

-- App range for resource utilization
CREATE MATERIALIZED VIEW resource_app_range AS
SELECT
  team_slug,
  app,
  environment,
  MIN(timestamp)::timestamptz AS "from",
  MAX(timestamp)::timestamptz AS "to"
FROM
  resource_utilization_metrics
GROUP BY
  team_slug, app, environment;

CREATE INDEX ON resource_app_range (team_slug, app, environment);

-- Resource utilization for team

CREATE MATERIALIZED VIEW resource_utilization_team AS
SELECT
  team_slug,
  environment,
  resource_type,
  timestamp,
  SUM(usage)::double precision AS usage,
  SUM(request)::double precision AS request
FROM
  resource_utilization_metrics
GROUP BY
  team_slug, environment, resource_type, timestamp;

CREATE INDEX ON resource_utilization_team (team_slug, environment, resource_type, timestamp);
