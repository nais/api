-- name: ListMembers :many
SELECT
	sqlc.embed(users),
	sqlc.embed(user_roles)
FROM
	user_roles
	JOIN teams ON teams.slug = user_roles.target_team_slug
	JOIN users ON users.id = user_roles.user_id
WHERE
	user_roles.target_team_slug = @team_slug::slug
ORDER BY
	CASE
		WHEN @order_by::TEXT = 'name:asc' THEN LOWER(users.name)
	END ASC,
	CASE
		WHEN @order_by::TEXT = 'name:desc' THEN LOWER(users.name)
	END DESC,
	CASE
		WHEN @order_by::TEXT = 'email:asc' THEN users.email
	END ASC,
	CASE
		WHEN @order_by::TEXT = 'email:desc' THEN users.email
	END DESC,
	CASE
		WHEN @order_by::TEXT = 'role:asc' THEN user_roles.role_name
	END ASC,
	CASE
		WHEN @order_by::TEXT = 'role:desc' THEN user_roles.role_name
	END DESC,
	users.name,
	users.email ASC
LIMIT
	sqlc.arg('limit')
OFFSET
	sqlc.arg('offset')
;

-- name: CountMembers :one
SELECT
	COUNT(user_roles.*)
FROM
	user_roles
	JOIN teams ON teams.slug = user_roles.target_team_slug
WHERE
	user_roles.target_team_slug = @team_slug
;

-- name: ListForUser :many
SELECT
	sqlc.embed(users),
	sqlc.embed(user_roles)
FROM
	user_roles
	JOIN teams ON teams.slug = user_roles.target_team_slug
	JOIN users ON users.id = user_roles.user_id
WHERE
	user_roles.user_id = @user_id
ORDER BY
	CASE
		WHEN @order_by::TEXT = 'slug:asc' THEN teams.slug
	END ASC,
	CASE
		WHEN @order_by::TEXT = 'slug:desc' THEN teams.slug
	END DESC,
	teams.slug ASC
LIMIT
	sqlc.arg('limit')
OFFSET
	sqlc.arg('offset')
;

-- name: CountForUser :one
SELECT
	COUNT(user_roles.*)
FROM
	user_roles
	JOIN teams ON teams.slug = user_roles.target_team_slug
WHERE
	user_roles.user_id = @user_id
;

-- name: GetMember :one
SELECT
	users.*,
	user_roles.role_name
FROM
	user_roles
	JOIN teams ON teams.slug = user_roles.target_team_slug
	JOIN users ON users.id = user_roles.user_id
WHERE
	user_roles.target_team_slug = @team_slug::slug
	AND user_roles.user_id = @user_id
;

-- name: GetMemberByEmail :one
SELECT
	users.*,
	user_roles.role_name
FROM
	user_roles
	JOIN teams ON teams.slug = user_roles.target_team_slug
	JOIN users ON users.id = user_roles.user_id
WHERE
	user_roles.target_team_slug = @team_slug::slug
	AND users.email = @email
;

-- name: AddMember :exec
INSERT INTO
	user_roles (user_id, role_name, target_team_slug)
VALUES
	(@user_id, @role_name, @team_slug::slug)
ON CONFLICT DO NOTHING
;

-- name: RemoveMember :exec
DELETE FROM user_roles
WHERE
	user_id = @user_id
	AND target_team_slug = @team_slug::slug
;

-- name: UserIsMember :one
SELECT
	EXISTS (
		SELECT
			id
		FROM
			user_roles
		WHERE
			user_id = @user_id
			AND target_team_slug = @team_slug::slug
			AND role_name IN ('Team member', 'Team owner')
	)
;

-- name: UserIsOwner :one
SELECT
	EXISTS (
		SELECT
			id
		FROM
			user_roles
		WHERE
			user_id = @user_id
			AND target_team_slug = @team_slug::slug
			AND role_name = 'Team owner'
	)
;
