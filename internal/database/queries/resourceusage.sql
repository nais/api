-- ResourceUtilizationRangeForTeam will return the min and max timestamps for a specific team.
-- name: ResourceUtilizationRangeForTeam :one
SELECT
    MIN(timestamp)::timestamptz AS "from",
    MAX(timestamp)::timestamptz AS "to"
FROM
    resource_utilization_metrics
WHERE
    team = $1;

-- ResourceUtilizationRangeForApp will return the min and max timestamps for a specific app.
-- name: ResourceUtilizationRangeForApp :one
SELECT
    MIN(timestamp)::timestamptz AS "from",
    MAX(timestamp)::timestamptz AS "to"
FROM
    resource_utilization_metrics
WHERE
    env = $1
    AND team = $2
    AND app = $3;

-- ResourceUtilizationOverageForTeam will return overage records for a given team, ordered by overage descending.
-- name: ResourceUtilizationOverageForTeam :many
SELECT
    usage,
    request,
    app,
    env,
    (request-usage)::double precision AS overage
FROM
    resource_utilization_metrics
WHERE
    team = $1
    AND timestamp = $2
    AND resource_type = $3
GROUP BY
    app, env, usage, request, timestamp
ORDER BY
    overage DESC;

-- ResourceUtilizationUpsert will insert or update resource utilization records.
-- name: ResourceUtilizationUpsert :batchexec
INSERT INTO resource_utilization_metrics (timestamp, env, team, app, resource_type, usage, request)
VALUES ($1, $2, $3, $4, $5, $6, $7)
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
    env = $1
    AND team = $2
    AND app = $3
    AND resource_type = $4
    AND timestamp >= sqlc.arg('start')::timestamptz
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
    env = $1
    AND team = $2
    AND resource_type = $3
    AND timestamp >= sqlc.arg('start')::timestamptz
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
    env = $1
    AND team = $2
    AND app = $3
    AND resource_type = $4
    AND timestamp = $5;

-- SpecificResourceUtilizationForTeam will return resource utilization for a team at a specific timestamp. Applications
-- with a usage greater than request will be ignored.
-- name: SpecificResourceUtilizationForTeam :one
SELECT
    SUM(usage)::double precision AS usage,
    SUM(request)::double precision AS request,
    timestamp
FROM
    resource_utilization_metrics
WHERE
    team = $1
    AND resource_type = $2
    AND timestamp = $3
    AND request > usage
GROUP BY
    timestamp;

-- AverageResourceUtilizationForTeam will return the average resource utilization for a team for a week.
-- name: AverageResourceUtilizationForTeam :one
SELECT
    (SUM(usage) / 24 / 7)::double precision AS usage,
    (SUM(request) / 24 / 7)::double precision AS request
FROM
    resource_utilization_metrics
WHERE
    team = $1
    AND resource_type = $2
    AND timestamp >= sqlc.arg('timestamp')::timestamptz - INTERVAL '1 week'
    AND timestamp < sqlc.arg('timestamp')::timestamptz
    AND request > usage;
