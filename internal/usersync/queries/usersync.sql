-- name: List :many
SELECT
	id,
	email,
	name,
	external_id
FROM
	users
ORDER BY
	name,
	email ASC
;

-- name: ListRoles :many
SELECT
	id,
	role_name,
	user_id,
	target_team_slug
FROM
	user_roles
ORDER BY
	role_name ASC
;

-- name: Create :one
INSERT INTO
	users (name, email, external_id)
VALUES
	(@name, LOWER(@email), @external_id)
RETURNING
	id,
	email,
	name,
	external_id
;

-- name: Update :exec
UPDATE users
SET
	name = @name,
	email = LOWER(@email),
	external_id = @external_id
WHERE
	id = @id
;

-- name: Delete :exec
DELETE FROM users
WHERE
	id = @id
;

-- name: AssignGlobalRole :exec
INSERT INTO
	user_roles (user_id, role_name)
VALUES
	(@user_id, @role_name)
ON CONFLICT DO NOTHING
;

-- name: RevokeGlobalRole :exec
DELETE FROM user_roles
WHERE
	user_id = @user_id
	AND target_team_slug IS NULL
	AND role_name = @role_name
;

-- name: ListLogEntriesByIDs :many
SELECT
	*
FROM
	usersync_log_entries
WHERE
	id = ANY (@ids::UUID[])
ORDER BY
	created_at DESC
;

-- name: ListLogEntries :many
SELECT
	*
FROM
	usersync_log_entries
ORDER BY
	created_at DESC
LIMIT
	sqlc.arg('limit')
OFFSET
	sqlc.arg('offset')
;

-- name: CountLogEntries :one
SELECT
	COUNT(*)
FROM
	usersync_log_entries
;

-- name: CreateLogEntry :exec
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
		@action,
		@user_id,
		@user_name,
		@user_email,
		@old_user_name,
		@old_user_email,
		@role_name
	)
;

-- name: ListGlobalAdmins :many
SELECT
	u.*
FROM
	users u
	INNER JOIN user_roles ur ON u.id = ur.user_id
WHERE
	ur.role_name = 'Admin'
	AND ur.target_team_slug IS NULL
ORDER BY
	u.name,
	u.email
;
