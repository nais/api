// Code generated by sqlc. DO NOT EDIT.
// source: usersync.sql

package usersyncsql

import (
	"context"

	"github.com/google/uuid"
)

const assignGlobalAdmin = `-- name: AssignGlobalAdmin :exec
UPDATE users
SET
	admin = TRUE
WHERE
	id = $1
`

func (q *Queries) AssignGlobalAdmin(ctx context.Context, id uuid.UUID) error {
	_, err := q.db.Exec(ctx, assignGlobalAdmin, id)
	return err
}

const assignGlobalRole = `-- name: AssignGlobalRole :exec
INSERT INTO
	user_roles (user_id, role_name)
VALUES
	($1, $2)
ON CONFLICT DO NOTHING
`

type AssignGlobalRoleParams struct {
	UserID   uuid.UUID
	RoleName string
}

func (q *Queries) AssignGlobalRole(ctx context.Context, arg AssignGlobalRoleParams) error {
	_, err := q.db.Exec(ctx, assignGlobalRole, arg.UserID, arg.RoleName)
	return err
}

const countLogEntries = `-- name: CountLogEntries :one
SELECT
	COUNT(*)
FROM
	usersync_log_entries
`

func (q *Queries) CountLogEntries(ctx context.Context) (int64, error) {
	row := q.db.QueryRow(ctx, countLogEntries)
	var count int64
	err := row.Scan(&count)
	return count, err
}

const create = `-- name: Create :one
INSERT INTO
	users (name, email, external_id, admin)
VALUES
	($1, LOWER($2), $3, FALSE)
RETURNING
	id, email, name, external_id, admin
`

type CreateParams struct {
	Name       string
	Email      string
	ExternalID string
}

func (q *Queries) Create(ctx context.Context, arg CreateParams) (*User, error) {
	row := q.db.QueryRow(ctx, create, arg.Name, arg.Email, arg.ExternalID)
	var i User
	err := row.Scan(
		&i.ID,
		&i.Email,
		&i.Name,
		&i.ExternalID,
		&i.Admin,
	)
	return &i, err
}

const createLogEntry = `-- name: CreateLogEntry :exec
INSERT INTO
	usersync_log_entries (
		action,
		user_id,
		user_name,
		user_email,
		old_user_name,
		old_user_email,
		role_name
	)
VALUES
	(
		$1,
		$2,
		$3,
		$4,
		$5,
		$6,
		$7
	)
`

type CreateLogEntryParams struct {
	Action       UsersyncLogEntryAction
	UserID       uuid.UUID
	UserName     string
	UserEmail    string
	OldUserName  *string
	OldUserEmail *string
	RoleName     *string
}

func (q *Queries) CreateLogEntry(ctx context.Context, arg CreateLogEntryParams) error {
	_, err := q.db.Exec(ctx, createLogEntry,
		arg.Action,
		arg.UserID,
		arg.UserName,
		arg.UserEmail,
		arg.OldUserName,
		arg.OldUserEmail,
		arg.RoleName,
	)
	return err
}

const delete = `-- name: Delete :exec
DELETE FROM users
WHERE
	id = $1
`

func (q *Queries) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := q.db.Exec(ctx, delete, id)
	return err
}

const list = `-- name: List :many
SELECT
	id, email, name, external_id, admin
FROM
	users
ORDER BY
	name,
	email ASC
`

func (q *Queries) List(ctx context.Context) ([]*User, error) {
	rows, err := q.db.Query(ctx, list)
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
			&i.Admin,
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

const listGlobalAdmins = `-- name: ListGlobalAdmins :many
SELECT
	u.id, u.email, u.name, u.external_id, u.admin
FROM
	users u
WHERE
	u.admin = TRUE
ORDER BY
	u.name,
	u.email
`

func (q *Queries) ListGlobalAdmins(ctx context.Context) ([]*User, error) {
	rows, err := q.db.Query(ctx, listGlobalAdmins)
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
			&i.Admin,
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

const listLogEntries = `-- name: ListLogEntries :many
SELECT
	id, created_at, action, user_id, user_name, user_email, old_user_name, old_user_email, role_name
FROM
	usersync_log_entries
ORDER BY
	created_at DESC
LIMIT
	$2
OFFSET
	$1
`

type ListLogEntriesParams struct {
	Offset int32
	Limit  int32
}

func (q *Queries) ListLogEntries(ctx context.Context, arg ListLogEntriesParams) ([]*UsersyncLogEntry, error) {
	rows, err := q.db.Query(ctx, listLogEntries, arg.Offset, arg.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []*UsersyncLogEntry{}
	for rows.Next() {
		var i UsersyncLogEntry
		if err := rows.Scan(
			&i.ID,
			&i.CreatedAt,
			&i.Action,
			&i.UserID,
			&i.UserName,
			&i.UserEmail,
			&i.OldUserName,
			&i.OldUserEmail,
			&i.RoleName,
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

const listLogEntriesByIDs = `-- name: ListLogEntriesByIDs :many
SELECT
	id, created_at, action, user_id, user_name, user_email, old_user_name, old_user_email, role_name
FROM
	usersync_log_entries
WHERE
	id = ANY ($1::UUID[])
ORDER BY
	created_at DESC
`

func (q *Queries) ListLogEntriesByIDs(ctx context.Context, ids []uuid.UUID) ([]*UsersyncLogEntry, error) {
	rows, err := q.db.Query(ctx, listLogEntriesByIDs, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []*UsersyncLogEntry{}
	for rows.Next() {
		var i UsersyncLogEntry
		if err := rows.Scan(
			&i.ID,
			&i.CreatedAt,
			&i.Action,
			&i.UserID,
			&i.UserName,
			&i.UserEmail,
			&i.OldUserName,
			&i.OldUserEmail,
			&i.RoleName,
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

const listRoles = `-- name: ListRoles :many
SELECT
	id,
	role_name,
	user_id,
	target_team_slug
FROM
	user_roles
ORDER BY
	role_name ASC
`

func (q *Queries) ListRoles(ctx context.Context) ([]*UserRole, error) {
	rows, err := q.db.Query(ctx, listRoles)
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

const revokeGlobalAdmin = `-- name: RevokeGlobalAdmin :exec
UPDATE users
SET
	admin = FALSE
WHERE
	id = $1
`

func (q *Queries) RevokeGlobalAdmin(ctx context.Context, id uuid.UUID) error {
	_, err := q.db.Exec(ctx, revokeGlobalAdmin, id)
	return err
}

const revokeGlobalRole = `-- name: RevokeGlobalRole :exec
DELETE FROM user_roles
WHERE
	user_id = $1
	AND target_team_slug IS NULL
	AND role_name = $2
`

type RevokeGlobalRoleParams struct {
	UserID   uuid.UUID
	RoleName string
}

func (q *Queries) RevokeGlobalRole(ctx context.Context, arg RevokeGlobalRoleParams) error {
	_, err := q.db.Exec(ctx, revokeGlobalRole, arg.UserID, arg.RoleName)
	return err
}

const update = `-- name: Update :exec
UPDATE users
SET
	name = $1,
	email = LOWER($2),
	external_id = $3
WHERE
	id = $4
`

type UpdateParams struct {
	Name       string
	Email      string
	ExternalID string
	ID         uuid.UUID
}

func (q *Queries) Update(ctx context.Context, arg UpdateParams) error {
	_, err := q.db.Exec(ctx, update,
		arg.Name,
		arg.Email,
		arg.ExternalID,
		arg.ID,
	)
	return err
}
