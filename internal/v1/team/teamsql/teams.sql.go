// Code generated by sqlc. DO NOT EDIT.
// source: teams.sql

package teamsql

import (
	"context"

	"github.com/nais/api/internal/slug"
)

const count = `-- name: Count :one
SELECT COUNT(*) FROM teams
`

func (q *Queries) Count(ctx context.Context) (int64, error) {
	row := q.db.QueryRow(ctx, count)
	var count int64
	err := row.Scan(&count)
	return count, err
}

const get = `-- name: Get :one
SELECT slug, purpose, last_successful_sync, slack_channel, google_group_email, azure_group_id, github_team_slug, gar_repository, cdn_bucket, delete_key_confirmed_at FROM teams
WHERE slug = $1
`

func (q *Queries) Get(ctx context.Context, argSlug slug.Slug) (*Team, error) {
	row := q.db.QueryRow(ctx, get, argSlug)
	var i Team
	err := row.Scan(
		&i.Slug,
		&i.Purpose,
		&i.LastSuccessfulSync,
		&i.SlackChannel,
		&i.GoogleGroupEmail,
		&i.AzureGroupID,
		&i.GithubTeamSlug,
		&i.GarRepository,
		&i.CdnBucket,
		&i.DeleteKeyConfirmedAt,
	)
	return &i, err
}

const list = `-- name: List :many
SELECT slug, purpose, last_successful_sync, slack_channel, google_group_email, azure_group_id, github_team_slug, gar_repository, cdn_bucket, delete_key_confirmed_at FROM teams
ORDER BY
    CASE WHEN $1::TEXT = 'slug:asc' THEN slug END ASC,
    CASE WHEN $1::TEXT = 'slug:desc' THEN slug END DESC,
    slug ASC
LIMIT $3
OFFSET $2
`

type ListParams struct {
	OrderBy string
	Offset  int32
	Limit   int32
}

func (q *Queries) List(ctx context.Context, arg ListParams) ([]*Team, error) {
	rows, err := q.db.Query(ctx, list, arg.OrderBy, arg.Offset, arg.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []*Team{}
	for rows.Next() {
		var i Team
		if err := rows.Scan(
			&i.Slug,
			&i.Purpose,
			&i.LastSuccessfulSync,
			&i.SlackChannel,
			&i.GoogleGroupEmail,
			&i.AzureGroupID,
			&i.GithubTeamSlug,
			&i.GarRepository,
			&i.CdnBucket,
			&i.DeleteKeyConfirmedAt,
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

const listBySlugs = `-- name: ListBySlugs :many
SELECT slug, purpose, last_successful_sync, slack_channel, google_group_email, azure_group_id, github_team_slug, gar_repository, cdn_bucket, delete_key_confirmed_at FROM teams
WHERE slug = ANY($1::slug[])
ORDER BY slug ASC
`

func (q *Queries) ListBySlugs(ctx context.Context, slugs []slug.Slug) ([]*Team, error) {
	rows, err := q.db.Query(ctx, listBySlugs, slugs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []*Team{}
	for rows.Next() {
		var i Team
		if err := rows.Scan(
			&i.Slug,
			&i.Purpose,
			&i.LastSuccessfulSync,
			&i.SlackChannel,
			&i.GoogleGroupEmail,
			&i.AzureGroupID,
			&i.GithubTeamSlug,
			&i.GarRepository,
			&i.CdnBucket,
			&i.DeleteKeyConfirmedAt,
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

const listEnvironmentsBySlugsAndEnvNames = `-- name: ListEnvironmentsBySlugsAndEnvNames :many
WITH input AS (
    SELECT
        unnest($1::slug[]) AS team_slug,
        unnest($2::text[]) AS environment
)
SELECT team_all_environments.team_slug, team_all_environments.environment, team_all_environments.gcp, team_all_environments.gcp_project_id, team_all_environments.id, team_all_environments.slack_alerts_channel
FROM team_all_environments
JOIN input ON input.team_slug = team_all_environments.team_slug
JOIN teams ON teams.slug = team_all_environments.team_slug
WHERE team_all_environments.environment = input.environment
ORDER BY team_all_environments.environment ASC
`

type ListEnvironmentsBySlugsAndEnvNamesParams struct {
	TeamSlugs    []slug.Slug
	Environments []string
}

// ListEnvironmentsBySlugsAndEnvNames returns a slice of team environments for a list of teams/envs, excluding
// deleted teams.
// Input is two arrays of equal length, one for slugs and one for names
func (q *Queries) ListEnvironmentsBySlugsAndEnvNames(ctx context.Context, arg ListEnvironmentsBySlugsAndEnvNamesParams) ([]*TeamAllEnvironment, error) {
	rows, err := q.db.Query(ctx, listEnvironmentsBySlugsAndEnvNames, arg.TeamSlugs, arg.Environments)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []*TeamAllEnvironment{}
	for rows.Next() {
		var i TeamAllEnvironment
		if err := rows.Scan(
			&i.TeamSlug,
			&i.Environment,
			&i.Gcp,
			&i.GcpProjectID,
			&i.ID,
			&i.SlackAlertsChannel,
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