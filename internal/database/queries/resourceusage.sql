-- ResourceUtilizationRangeForTeam will return the min and max timestamps for a specific team.
-- name: ResourceUtilizationRangeForTeam :one
SELECT "from", "to" FROM resource_team_range WHERE team_slug = @team_slug;

-- ResourceUtilizationRangeForApp will return the min and max timestamps for a specific app.
-- name: ResourceUtilizationRangeForApp :one
SELECT
    MIN(timestamp)::timestamptz AS "from",
    MAX(timestamp)::timestamptz AS "to"
FROM
    resource_utilization_metrics
WHERE
    environment = @environment
    AND team_slug = @team_slug
    AND app = @app;

-- ResourceUtilizationOverageForTeam will return overage records for a given team, ordered by overage descending.
-- name: ResourceUtilizationOverageForTeam :many
SELECT
    usage,
    request,
    app,
    environment,
    (request-usage)::double precision AS overage
FROM
    resource_utilization_metrics
WHERE
    team_slug = @team_slug
    AND timestamp = @timestamp
    AND resource_type = @resource_type
GROUP BY
    app, environment, usage, request, timestamp
ORDER BY
    overage DESC;

-- ResourceUtilizationUpsert will insert or update resource utilization records.
-- name: ResourceUtilizationUpsert :batchexec
INSERT INTO resource_utilization_metrics (timestamp, environment, team_slug, app, resource_type, usage, request)
VALUES (@timestamp, @environment, @team_slug, @app, @resource_type, @usage, @request)
ON CONFLICT ON CONSTRAINT resource_utilization_metric DO NOTHING;

-- MaxResourceUtilizationDate will return the max date for resource utilization records.
-- name: MaxResourceUtilizationDate :one
SELECT MAX(timestamp)::timestamptz FROM resource_utilization_metrics;

-- ResourceUtilizationForApp will return resource utilization records for a given app.
-- name: ResourceUtilizationForApp :many
SELECT
    *
FROM
    resource_utilization_metrics
WHERE
    environment = @environment
    AND team_slug = @team_slug
    AND app = @app
    AND resource_type = @resource_type
    AND timestamp >= @start::timestamptz
    AND timestamp < sqlc.arg('end')::timestamptz
ORDER BY
    timestamp ASC;

-- ResourceUtilizationForTeam will return resource utilization records for a given team.
-- name: ResourceUtilizationForTeam :many
SELECT
    SUM(usage)::double precision AS usage,
    SUM(request)::double precision AS request,
    timestamp
FROM
    resource_utilization_metrics
WHERE
    environment = @environment
    AND team_slug = @team_slug
    AND resource_type = @resource_type
    AND timestamp >= @start::timestamptz
    AND timestamp < sqlc.arg('end')::timestamptz
GROUP BY
    timestamp
ORDER BY
    timestamp ASC;

-- SpecificResourceUtilizationForApp will return resource utilization for an app at a specific timestamp.
-- name: SpecificResourceUtilizationForApp :one
SELECT
    usage,
    request,
    timestamp
FROM
    resource_utilization_metrics
WHERE
    environment = @environment
    AND team_slug = @team_slug
    AND app = @app
    AND resource_type = @resource_type
    AND timestamp = @timestamp;

-- SpecificResourceUtilizationForTeam will return resource utilization for a team at a specific timestamp. Applications
-- with a usage greater than request will be ignored.
-- name: SpecificResourceUtilizationForTeam :many
SELECT
    COALESCE(SUM(usage),0)::double precision AS usage,
    COALESCE(SUM(request),0)::double precision AS request,
    timestamp,
    request > usage as usable_for_cost
FROM
    resource_utilization_metrics
WHERE
    team_slug = @team_slug
    AND resource_type = @resource_type
    AND timestamp = @timestamp
GROUP BY
    timestamp, usable_for_cost
ORDER BY usable_for_cost DESC;

-- AverageResourceUtilizationForTeam will return the average resource utilization for a team for a week.
-- name: AverageResourceUtilizationForTeam :one
SELECT
    (COALESCE(SUM(usage),0) / 24 / 7)::double precision AS usage,
    (COALESCE(SUM(request),0) / 24 / 7)::double precision AS request
FROM
    resource_utilization_metrics
WHERE
    team_slug = @team_slug
    AND resource_type = @resource_type
    AND timestamp >= sqlc.arg(timestamp)::timestamptz - INTERVAL '1 week'
    AND timestamp < sqlc.arg(timestamp)::timestamptz
    AND request > usage;

-- Refresh materialized view resource_team_range
-- name: RefreshResourceTeamRange :exec
REFRESH MATERIALIZED VIEW CONCURRENTLY resource_team_range;
