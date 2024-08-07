// Code generated by sqlc. DO NOT EDIT.
// source: resourceusage.sql

package gensql

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nais/api/internal/slug"
)

const averageResourceUtilizationForTeam = `-- name: AverageResourceUtilizationForTeam :one
SELECT
    (COALESCE(SUM(usage),0) / 24 / 7)::double precision AS usage,
    (COALESCE(SUM(request),0) / 24 / 7)::double precision AS request
FROM
    resource_utilization_metrics
WHERE
    team_slug = $1
    AND resource_type = $2
    AND timestamp >= $3::timestamptz - INTERVAL '1 week'
    AND timestamp < $3::timestamptz
    AND request > usage
`

type AverageResourceUtilizationForTeamParams struct {
	TeamSlug     slug.Slug
	ResourceType ResourceType
	Timestamp    pgtype.Timestamptz
}

type AverageResourceUtilizationForTeamRow struct {
	Usage   float64
	Request float64
}

// AverageResourceUtilizationForTeam will return the average resource utilization for a team for a week.
func (q *Queries) AverageResourceUtilizationForTeam(ctx context.Context, arg AverageResourceUtilizationForTeamParams) (*AverageResourceUtilizationForTeamRow, error) {
	row := q.db.QueryRow(ctx, averageResourceUtilizationForTeam, arg.TeamSlug, arg.ResourceType, arg.Timestamp)
	var i AverageResourceUtilizationForTeamRow
	err := row.Scan(&i.Usage, &i.Request)
	return &i, err
}

const maxResourceUtilizationDate = `-- name: MaxResourceUtilizationDate :one
SELECT MAX(timestamp)::timestamptz FROM resource_utilization_metrics
`

// MaxResourceUtilizationDate will return the max date for resource utilization records.
func (q *Queries) MaxResourceUtilizationDate(ctx context.Context) (pgtype.Timestamptz, error) {
	row := q.db.QueryRow(ctx, maxResourceUtilizationDate)
	var column_1 pgtype.Timestamptz
	err := row.Scan(&column_1)
	return column_1, err
}

const refreshResourceTeamRange = `-- name: RefreshResourceTeamRange :exec
REFRESH MATERIALIZED VIEW CONCURRENTLY resource_team_range
`

// Refresh materialized view resource_team_range
func (q *Queries) RefreshResourceTeamRange(ctx context.Context) error {
	_, err := q.db.Exec(ctx, refreshResourceTeamRange)
	return err
}

const resourceUtilizationForApp = `-- name: ResourceUtilizationForApp :many
SELECT
    id, timestamp, environment, team_slug, app, resource_type, usage, request
FROM
    resource_utilization_metrics
WHERE
    environment = $1
    AND team_slug = $2
    AND app = $3
    AND resource_type = $4
    AND timestamp >= $5::timestamptz
    AND timestamp < $6::timestamptz
ORDER BY
    timestamp ASC
`

type ResourceUtilizationForAppParams struct {
	Environment  string
	TeamSlug     slug.Slug
	App          string
	ResourceType ResourceType
	Start        pgtype.Timestamptz
	End          pgtype.Timestamptz
}

// ResourceUtilizationForApp will return resource utilization records for a given app.
func (q *Queries) ResourceUtilizationForApp(ctx context.Context, arg ResourceUtilizationForAppParams) ([]*ResourceUtilizationMetric, error) {
	rows, err := q.db.Query(ctx, resourceUtilizationForApp,
		arg.Environment,
		arg.TeamSlug,
		arg.App,
		arg.ResourceType,
		arg.Start,
		arg.End,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []*ResourceUtilizationMetric{}
	for rows.Next() {
		var i ResourceUtilizationMetric
		if err := rows.Scan(
			&i.ID,
			&i.Timestamp,
			&i.Environment,
			&i.TeamSlug,
			&i.App,
			&i.ResourceType,
			&i.Usage,
			&i.Request,
		); err != nil {
			return nil, err
		}
		items = append(items, &i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const resourceUtilizationForTeam = `-- name: ResourceUtilizationForTeam :many
SELECT
    SUM(usage)::double precision AS usage,
    SUM(request)::double precision AS request,
    timestamp
FROM
    resource_utilization_metrics
WHERE
    environment = $1
    AND team_slug = $2
    AND resource_type = $3
    AND timestamp >= $4::timestamptz
    AND timestamp < $5::timestamptz
GROUP BY
    timestamp
ORDER BY
    timestamp ASC
`

type ResourceUtilizationForTeamParams struct {
	Environment  string
	TeamSlug     slug.Slug
	ResourceType ResourceType
	Start        pgtype.Timestamptz
	End          pgtype.Timestamptz
}

type ResourceUtilizationForTeamRow struct {
	Usage     float64
	Request   float64
	Timestamp pgtype.Timestamptz
}

// ResourceUtilizationForTeam will return resource utilization records for a given team.
func (q *Queries) ResourceUtilizationForTeam(ctx context.Context, arg ResourceUtilizationForTeamParams) ([]*ResourceUtilizationForTeamRow, error) {
	rows, err := q.db.Query(ctx, resourceUtilizationForTeam,
		arg.Environment,
		arg.TeamSlug,
		arg.ResourceType,
		arg.Start,
		arg.End,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []*ResourceUtilizationForTeamRow{}
	for rows.Next() {
		var i ResourceUtilizationForTeamRow
		if err := rows.Scan(&i.Usage, &i.Request, &i.Timestamp); err != nil {
			return nil, err
		}
		items = append(items, &i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const resourceUtilizationOverageForTeam = `-- name: ResourceUtilizationOverageForTeam :many
SELECT
    usage,
    request,
    app,
    environment,
    (request-usage)::double precision AS overage
FROM
    resource_utilization_metrics
WHERE
    team_slug = $1
    AND timestamp = $2
    AND resource_type = $3
GROUP BY
    app, environment, usage, request, timestamp
ORDER BY
    overage DESC
`

type ResourceUtilizationOverageForTeamParams struct {
	TeamSlug     slug.Slug
	Timestamp    pgtype.Timestamptz
	ResourceType ResourceType
}

type ResourceUtilizationOverageForTeamRow struct {
	Usage       float64
	Request     float64
	App         string
	Environment string
	Overage     float64
}

// ResourceUtilizationOverageForTeam will return overage records for a given team, ordered by overage descending.
func (q *Queries) ResourceUtilizationOverageForTeam(ctx context.Context, arg ResourceUtilizationOverageForTeamParams) ([]*ResourceUtilizationOverageForTeamRow, error) {
	rows, err := q.db.Query(ctx, resourceUtilizationOverageForTeam, arg.TeamSlug, arg.Timestamp, arg.ResourceType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []*ResourceUtilizationOverageForTeamRow{}
	for rows.Next() {
		var i ResourceUtilizationOverageForTeamRow
		if err := rows.Scan(
			&i.Usage,
			&i.Request,
			&i.App,
			&i.Environment,
			&i.Overage,
		); err != nil {
			return nil, err
		}
		items = append(items, &i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const resourceUtilizationRangeForApp = `-- name: ResourceUtilizationRangeForApp :one
SELECT
    MIN(timestamp)::timestamptz AS "from",
    MAX(timestamp)::timestamptz AS "to"
FROM
    resource_utilization_metrics
WHERE
    environment = $1
    AND team_slug = $2
    AND app = $3
`

type ResourceUtilizationRangeForAppParams struct {
	Environment string
	TeamSlug    slug.Slug
	App         string
}

type ResourceUtilizationRangeForAppRow struct {
	From pgtype.Timestamptz
	To   pgtype.Timestamptz
}

// ResourceUtilizationRangeForApp will return the min and max timestamps for a specific app.
func (q *Queries) ResourceUtilizationRangeForApp(ctx context.Context, arg ResourceUtilizationRangeForAppParams) (*ResourceUtilizationRangeForAppRow, error) {
	row := q.db.QueryRow(ctx, resourceUtilizationRangeForApp, arg.Environment, arg.TeamSlug, arg.App)
	var i ResourceUtilizationRangeForAppRow
	err := row.Scan(&i.From, &i.To)
	return &i, err
}

const resourceUtilizationRangeForTeam = `-- name: ResourceUtilizationRangeForTeam :one
SELECT "from", "to" FROM resource_team_range WHERE team_slug = $1
`

type ResourceUtilizationRangeForTeamRow struct {
	From pgtype.Timestamptz
	To   pgtype.Timestamptz
}

// ResourceUtilizationRangeForTeam will return the min and max timestamps for a specific team.
func (q *Queries) ResourceUtilizationRangeForTeam(ctx context.Context, teamSlug slug.Slug) (*ResourceUtilizationRangeForTeamRow, error) {
	row := q.db.QueryRow(ctx, resourceUtilizationRangeForTeam, teamSlug)
	var i ResourceUtilizationRangeForTeamRow
	err := row.Scan(&i.From, &i.To)
	return &i, err
}

const specificResourceUtilizationForApp = `-- name: SpecificResourceUtilizationForApp :one
SELECT
    usage,
    request,
    timestamp
FROM
    resource_utilization_metrics
WHERE
    environment = $1
    AND team_slug = $2
    AND app = $3
    AND resource_type = $4
    AND timestamp = $5
`

type SpecificResourceUtilizationForAppParams struct {
	Environment  string
	TeamSlug     slug.Slug
	App          string
	ResourceType ResourceType
	Timestamp    pgtype.Timestamptz
}

type SpecificResourceUtilizationForAppRow struct {
	Usage     float64
	Request   float64
	Timestamp pgtype.Timestamptz
}

// SpecificResourceUtilizationForApp will return resource utilization for an app at a specific timestamp.
func (q *Queries) SpecificResourceUtilizationForApp(ctx context.Context, arg SpecificResourceUtilizationForAppParams) (*SpecificResourceUtilizationForAppRow, error) {
	row := q.db.QueryRow(ctx, specificResourceUtilizationForApp,
		arg.Environment,
		arg.TeamSlug,
		arg.App,
		arg.ResourceType,
		arg.Timestamp,
	)
	var i SpecificResourceUtilizationForAppRow
	err := row.Scan(&i.Usage, &i.Request, &i.Timestamp)
	return &i, err
}

const specificResourceUtilizationForTeam = `-- name: SpecificResourceUtilizationForTeam :many
SELECT
    COALESCE(SUM(usage),0)::double precision AS usage,
    COALESCE(SUM(request),0)::double precision AS request,
    timestamp,
    request > usage as usable_for_cost
FROM
    resource_utilization_metrics
WHERE
    team_slug = $1
    AND resource_type = $2
    AND timestamp = $3
GROUP BY
    timestamp, usable_for_cost
ORDER BY usable_for_cost DESC
`

type SpecificResourceUtilizationForTeamParams struct {
	TeamSlug     slug.Slug
	ResourceType ResourceType
	Timestamp    pgtype.Timestamptz
}

type SpecificResourceUtilizationForTeamRow struct {
	Usage         float64
	Request       float64
	Timestamp     pgtype.Timestamptz
	UsableForCost bool
}

// SpecificResourceUtilizationForTeam will return resource utilization for a team at a specific timestamp. Applications
// with a usage greater than request will be ignored.
func (q *Queries) SpecificResourceUtilizationForTeam(ctx context.Context, arg SpecificResourceUtilizationForTeamParams) ([]*SpecificResourceUtilizationForTeamRow, error) {
	rows, err := q.db.Query(ctx, specificResourceUtilizationForTeam, arg.TeamSlug, arg.ResourceType, arg.Timestamp)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []*SpecificResourceUtilizationForTeamRow{}
	for rows.Next() {
		var i SpecificResourceUtilizationForTeamRow
		if err := rows.Scan(
			&i.Usage,
			&i.Request,
			&i.Timestamp,
			&i.UsableForCost,
		); err != nil {
			return nil, err
		}
		items = append(items, &i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}
