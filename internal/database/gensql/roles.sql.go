// Code generated by sqlc. DO NOT EDIT.
// source: roles.sql

package gensql

import (
	"context"

	"github.com/google/uuid"
	"github.com/nais/api/internal/slug"
)

const assignGlobalRoleToServiceAccount = `-- name: AssignGlobalRoleToServiceAccount :exec
INSERT INTO service_account_roles (service_account_id, role_name)
VALUES ($1, $2) ON CONFLICT DO NOTHING
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
INSERT INTO user_roles (user_id, role_name)
VALUES ($1, $2) ON CONFLICT DO NOTHING
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
INSERT INTO service_account_roles (service_account_id, role_name, target_team_slug)
VALUES ($1, $2, $3::slug) ON CONFLICT DO NOTHING
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
INSERT INTO user_roles (user_id, role_name, target_team_slug)
VALUES ($1, $2, $3::slug) ON CONFLICT DO NOTHING
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

const getAllUserRoles = `-- name: GetAllUserRoles :many
SELECT id, role_name, user_id, target_team_slug, target_service_account_id FROM user_roles
ORDER BY role_name ASC
`

func (q *Queries) GetAllUserRoles(ctx context.Context) ([]*UserRole, error) {
	rows, err := q.db.Query(ctx, getAllUserRoles)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []*UserRole{}
	for rows.Next() {
		var i UserRole
		if err := rows.Scan(
			&i.ID,
			&i.RoleName,
			&i.UserID,
			&i.TargetTeamSlug,
			&i.TargetServiceAccountID,
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

const getUserRoles = `-- name: GetUserRoles :many
SELECT id, role_name, user_id, target_team_slug, target_service_account_id FROM user_roles
WHERE user_id = $1
ORDER BY role_name ASC
`

func (q *Queries) GetUserRoles(ctx context.Context, userID uuid.UUID) ([]*UserRole, error) {
	rows, err := q.db.Query(ctx, getUserRoles, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []*UserRole{}
	for rows.Next() {
		var i UserRole
		if err := rows.Scan(
			&i.ID,
			&i.RoleName,
			&i.UserID,
			&i.TargetTeamSlug,
			&i.TargetServiceAccountID,
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

const getUserRolesForUsers = `-- name: GetUserRolesForUsers :many
SELECT user_id, role_name, target_team_slug, target_service_account_id
FROM user_roles
WHERE user_id = ANY($1::uuid[])
ORDER BY user_id
`

type GetUserRolesForUsersRow struct {
	UserID                 uuid.UUID
	RoleName               RoleName
	TargetTeamSlug         *slug.Slug
	TargetServiceAccountID *uuid.UUID
}

func (q *Queries) GetUserRolesForUsers(ctx context.Context, userIds []uuid.UUID) ([]*GetUserRolesForUsersRow, error) {
	rows, err := q.db.Query(ctx, getUserRolesForUsers, userIds)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []*GetUserRolesForUsersRow{}
	for rows.Next() {
		var i GetUserRolesForUsersRow
		if err := rows.Scan(
			&i.UserID,
			&i.RoleName,
			&i.TargetTeamSlug,
			&i.TargetServiceAccountID,
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

const getUsersWithGloballyAssignedRole = `-- name: GetUsersWithGloballyAssignedRole :many
SELECT users.id, users.email, users.name, users.external_id FROM users
JOIN user_roles ON user_roles.user_id = users.id
WHERE user_roles.target_team_slug IS NULL
AND user_roles.target_service_account_id IS NULL
AND user_roles.role_name = $1
ORDER BY name, email ASC
`

func (q *Queries) GetUsersWithGloballyAssignedRole(ctx context.Context, roleName RoleName) ([]*User, error) {
	rows, err := q.db.Query(ctx, getUsersWithGloballyAssignedRole, roleName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []*User{}
	for rows.Next() {
		var i User
		if err := rows.Scan(
			&i.ID,
			&i.Email,
			&i.Name,
			&i.ExternalID,
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

const removeAllServiceAccountRoles = `-- name: RemoveAllServiceAccountRoles :exec
DELETE FROM service_account_roles
WHERE service_account_id = $1
`

func (q *Queries) RemoveAllServiceAccountRoles(ctx context.Context, serviceAccountID uuid.UUID) error {
	_, err := q.db.Exec(ctx, removeAllServiceAccountRoles, serviceAccountID)
	return err
}

const revokeGlobalUserRole = `-- name: RevokeGlobalUserRole :exec
DELETE FROM user_roles
WHERE user_id = $1
AND target_team_slug IS NULL
AND target_service_account_id IS NULL
AND role_name = $2
`

type RevokeGlobalUserRoleParams struct {
	UserID   uuid.UUID
	RoleName RoleName
}

func (q *Queries) RevokeGlobalUserRole(ctx context.Context, arg RevokeGlobalUserRoleParams) error {
	_, err := q.db.Exec(ctx, revokeGlobalUserRole, arg.UserID, arg.RoleName)
	return err
}
