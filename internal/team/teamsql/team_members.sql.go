// Code generated by sqlc. DO NOT EDIT.
// source: team_members.sql

package teamsql

import (
	"context"

	"github.com/nais/api/internal/slug"
)

const countMembers = `-- name: CountMembers :one
SELECT COUNT(user_roles.*)
FROM user_roles
JOIN teams ON teams.slug = user_roles.target_team_slug
WHERE user_roles.target_team_slug = $1
`

// GetTeamMembersCount returns the total number of team members of a non-deleted team.
func (q *Queries) CountMembers(ctx context.Context, teamSlug *slug.Slug) (int64, error) {
	row := q.db.QueryRow(ctx, countMembers, teamSlug)
	var count int64
	err := row.Scan(&count)
	return count, err
}

const listMembers = `-- name: ListMembers :many
SELECT users.id, users.email, users.name, users.external_id, user_roles.id, user_roles.role_name, user_roles.user_id, user_roles.target_team_slug, user_roles.target_service_account_id
FROM user_roles
JOIN teams ON teams.slug = user_roles.target_team_slug
JOIN users ON users.id = user_roles.user_id
WHERE user_roles.target_team_slug = $1::slug
ORDER BY
    CASE WHEN $2::TEXT = 'name:asc' THEN users.name END ASC,
    CASE WHEN $2::TEXT = 'name:desc' THEN users.name END DESC,
    CASE WHEN $2::TEXT = 'email:asc' THEN users.email END ASC,
    CASE WHEN $2::TEXT = 'email:desc' THEN users.email END DESC,
    CASE WHEN $2::TEXT = 'role:asc' THEN user_roles.role_name END ASC,
    CASE WHEN $2::TEXT = 'role:desc' THEN user_roles.role_name END DESC,
    users.name,
    users.email ASC
LIMIT $4
OFFSET $3
`

type ListMembersParams struct {
	TeamSlug slug.Slug
	OrderBy  string
	Offset   int32
	Limit    int32
}

type ListMembersRow struct {
	User     User
	UserRole UserRole
}

// GetTeamMembers returns a slice of team members of a non-deleted team.
func (q *Queries) ListMembers(ctx context.Context, arg ListMembersParams) ([]*ListMembersRow, error) {
	rows, err := q.db.Query(ctx, listMembers,
		arg.TeamSlug,
		arg.OrderBy,
		arg.Offset,
		arg.Limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []*ListMembersRow{}
	for rows.Next() {
		var i ListMembersRow
		if err := rows.Scan(
			&i.User.ID,
			&i.User.Email,
			&i.User.Name,
			&i.User.ExternalID,
			&i.UserRole.ID,
			&i.UserRole.RoleName,
			&i.UserRole.UserID,
			&i.UserRole.TargetTeamSlug,
			&i.UserRole.TargetServiceAccountID,
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