-- +goose Up

-- Team range for resource utilization
DROP MATERIALIZED VIEW IF EXISTS resource_team_range;
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
DROP MATERIALIZED VIEW IF EXISTS resource_app_range;
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

CREATE INDEX ON resource_app_range (environment, team_slug, app);

-- Resource utilization for team

DROP MATERIALIZED VIEW IF EXISTS resource_utilization_team;
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

CREATE INDEX ON resource_utilization_team (timestamp DESC, team_slug, environment, resource_type);
