// Code generated by sqlc. DO NOT EDIT.
// source: repository.sql

package repositorysql

import (
	"context"

	"github.com/nais/api/internal/slug"
)

const countForTeam = `-- name: CountForTeam :one
SELECT
	COUNT(*)
FROM
	team_repositories
WHERE
	team_slug = $1
	AND CASE
		WHEN $2::TEXT IS NOT NULL THEN github_repository ILIKE '%' || $2 || '%'
		ELSE TRUE
	END
`

type CountForTeamParams struct {
	TeamSlug slug.Slug
	Search   *string
}

func (q *Queries) CountForTeam(ctx context.Context, arg CountForTeamParams) (int64, error) {
	row := q.db.QueryRow(ctx, countForTeam, arg.TeamSlug, arg.Search)
	var count int64
	err := row.Scan(&count)
	return count, err
}

const create = `-- name: Create :one
INSERT INTO
	team_repositories (team_slug, github_repository)
VALUES
	($1, $2)
RETURNING
	team_slug, github_repository
`

type CreateParams struct {
	TeamSlug         slug.Slug
	GithubRepository string
}

func (q *Queries) Create(ctx context.Context, arg CreateParams) (*TeamRepository, error) {
	row := q.db.QueryRow(ctx, create, arg.TeamSlug, arg.GithubRepository)
	var i TeamRepository
	err := row.Scan(&i.TeamSlug, &i.GithubRepository)
	return &i, err
}

const listForTeam = `-- name: ListForTeam :many
SELECT
	team_slug, github_repository
FROM
	team_repositories
WHERE
	team_slug = $1
	AND CASE
		WHEN $2::TEXT IS NOT NULL THEN github_repository ILIKE '%' || $2 || '%'
		ELSE TRUE
	END
ORDER BY
	github_repository ASC
LIMIT
	$4
OFFSET
	$3
`

type ListForTeamParams struct {
	TeamSlug slug.Slug
	Search   *string
	Offset   int32
	Limit    int32
}

func (q *Queries) ListForTeam(ctx context.Context, arg ListForTeamParams) ([]*TeamRepository, error) {
	rows, err := q.db.Query(ctx, listForTeam,
		arg.TeamSlug,
		arg.Search,
		arg.Offset,
		arg.Limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []*TeamRepository{}
	for rows.Next() {
		var i TeamRepository
		if err := rows.Scan(&i.TeamSlug, &i.GithubRepository); err != nil {
			return nil, err
		}
		items = append(items, &i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const remove = `-- name: Remove :exec
DELETE FROM team_repositories
WHERE
	team_slug = $1
	AND github_repository = $2
`

type RemoveParams struct {
	TeamSlug         slug.Slug
	GithubRepository string
}

func (q *Queries) Remove(ctx context.Context, arg RemoveParams) error {
	_, err := q.db.Exec(ctx, remove, arg.TeamSlug, arg.GithubRepository)
	return err
}