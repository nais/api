// Code generated by sqlc. DO NOT EDIT.
// source: roles.sql

package rolesql

import (
	"context"

	"github.com/google/uuid"
	"github.com/nais/api/internal/slug"
)

const assignGlobalRoleToServiceAccount = `-- name: AssignGlobalRoleToServiceAccount :exec
INSERT INTO
	service_account_roles (service_account_id, role_name)
VALUES
	($1, $2)
ON CONFLICT DO NOTHING
`

type AssignGlobalRoleToServiceAccountParams struct {
	ServiceAccountID uuid.UUID
	RoleName         RoleName
}

func (q *Queries) AssignGlobalRoleToServiceAccount(ctx context.Context, arg AssignGlobalRoleToServiceAccountParams) error {
	_, err := q.db.Exec(ctx, assignGlobalRoleToServiceAccount, arg.ServiceAccountID, arg.RoleName)
	return err
}

const assignGlobalRoleToUser = `-- name: AssignGlobalRoleToUser :exec
INSERT INTO
	user_roles (user_id, role_name)
VALUES
	($1, $2)
ON CONFLICT DO NOTHING
`

type AssignGlobalRoleToUserParams struct {
	UserID   uuid.UUID
	RoleName RoleName
}

func (q *Queries) AssignGlobalRoleToUser(ctx context.Context, arg AssignGlobalRoleToUserParams) error {
	_, err := q.db.Exec(ctx, assignGlobalRoleToUser, arg.UserID, arg.RoleName)
	return err
}

const assignTeamRoleToServiceAccount = `-- name: AssignTeamRoleToServiceAccount :exec
INSERT INTO
	service_account_roles (service_account_id, role_name, target_team_slug)
VALUES
	(
		$1,
		$2,
		$3::slug
	)
ON CONFLICT DO NOTHING
`

type AssignTeamRoleToServiceAccountParams struct {
	ServiceAccountID uuid.UUID
	RoleName         RoleName
	TargetTeamSlug   slug.Slug
}

func (q *Queries) AssignTeamRoleToServiceAccount(ctx context.Context, arg AssignTeamRoleToServiceAccountParams) error {
	_, err := q.db.Exec(ctx, assignTeamRoleToServiceAccount, arg.ServiceAccountID, arg.RoleName, arg.TargetTeamSlug)
	return err
}

const assignTeamRoleToUser = `-- name: AssignTeamRoleToUser :exec
INSERT INTO
	user_roles (user_id, role_name, target_team_slug)
VALUES
	($1, $2, $3::slug)
ON CONFLICT DO NOTHING
`

type AssignTeamRoleToUserParams struct {
	UserID         uuid.UUID
	RoleName       RoleName
	TargetTeamSlug slug.Slug
}

func (q *Queries) AssignTeamRoleToUser(ctx context.Context, arg AssignTeamRoleToUserParams) error {
	_, err := q.db.Exec(ctx, assignTeamRoleToUser, arg.UserID, arg.RoleName, arg.TargetTeamSlug)
	return err
}

const getRolesForServiceAccounts = `-- name: GetRolesForServiceAccounts :many
SELECT
	service_account_id,
	JSON_AGG(
		JSON_BUILD_OBJECT(
			'role_name',
			role_name,
			'target_team_slug',
			target_team_slug,
			'target_service_account_id',
			target_service_account_id
		)
	) AS roles
FROM
	service_account_roles
WHERE
	service_account_id = ANY ($1::UUID [])
GROUP BY
	service_account_id
ORDER BY
	service_account_id
`

type GetRolesForServiceAccountsRow struct {
	ServiceAccountID uuid.UUID
	Roles            []byte
}

func (q *Queries) GetRolesForServiceAccounts(ctx context.Context, serviceAccountIds []uuid.UUID) ([]*GetRolesForServiceAccountsRow, error) {
	rows, err := q.db.Query(ctx, getRolesForServiceAccounts, serviceAccountIds)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []*GetRolesForServiceAccountsRow{}
	for rows.Next() {
		var i GetRolesForServiceAccountsRow
		if err := rows.Scan(&i.ServiceAccountID, &i.Roles); err != nil {
			return nil, err
		}
		items = append(items, &i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const getRolesForUsers = `-- name: GetRolesForUsers :many
SELECT
	user_id,
	JSON_AGG(
		JSON_BUILD_OBJECT(
			'role_name',
			role_name,
			'target_team_slug',
			target_team_slug,
			'target_service_account_id',
			target_service_account_id
		)
	) AS roles
FROM
	user_roles
WHERE
	user_id = ANY ($1::UUID [])
GROUP BY
	user_id
ORDER BY
	user_id
`

type GetRolesForUsersRow struct {
	UserID uuid.UUID
	Roles  []byte
}

func (q *Queries) GetRolesForUsers(ctx context.Context, userIds []uuid.UUID) ([]*GetRolesForUsersRow, error) {
	rows, err := q.db.Query(ctx, getRolesForUsers, userIds)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []*GetRolesForUsersRow{}
	for rows.Next() {
		var i GetRolesForUsersRow
		if err := rows.Scan(&i.UserID, &i.Roles); err != nil {
			return nil, err
		}
		items = append(items, &i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}