-- name: ListRoles :many
SELECT
	*
FROM
	roles
ORDER BY
	name ASC
OFFSET
	sqlc.arg('offset')
LIMIT
	sqlc.arg('limit')
;

-- name: CountRoles :one
SELECT
	COUNT(*)
FROM
	roles
;

-- name: ListRolesForServiceAccount :many
SELECT
	roles.*
FROM
	roles
	JOIN service_account_roles sar ON roles.name = sar.role_name
WHERE
	sar.service_account_id = @service_account_id
ORDER BY
	roles.name
OFFSET
	sqlc.arg('offset')
LIMIT
	sqlc.arg('limit')
;

-- name: CountRolesForServiceAccount :one
SELECT
	COUNT(*)
FROM
	service_account_roles
WHERE
	service_account_id = @service_account_id
;

-- name: GetRoleByName :one
SELECT
	*
FROM
	roles
WHERE
	name = @name
;

-- name: AssignTeamRoleToUser :exec
INSERT INTO
	user_roles (user_id, role_name, target_team_slug)
VALUES
	(@user_id, @role_name, @target_team_slug::slug)
ON CONFLICT DO NOTHING
;

-- TODO: This should be rewritten to fetch rows from the roles table instead as it uses the authz.Role struct, which reflects rows from the roles table.
-- name: GetRolesForUsers :many
SELECT
	user_id,
	JSON_AGG(
		JSON_BUILD_OBJECT(
			'name',
			role_name,
			'target_team_slug',
			target_team_slug
		)
	) AS roles
FROM
	user_roles
WHERE
	user_id = ANY (@user_ids::UUID[])
GROUP BY
	user_id
ORDER BY
	user_id
;

-- name: GetRolesForServiceAccounts :many
SELECT
	service_account_id,
	JSON_AGG(
		JSON_BUILD_OBJECT(
			'role_name',
			role_name,
			'target_team_slug',
			service_accounts.team_slug
		)
	) AS roles
FROM
	service_account_roles
	JOIN service_accounts ON service_accounts.id = service_account_roles.service_account_id
WHERE
	service_account_id = ANY (@service_account_ids::UUID[])
GROUP BY
	service_account_id
ORDER BY
	service_account_id
;

-- name: AssignGlobalRoleToUser :exec
INSERT INTO
	user_roles (user_id, role_name)
VALUES
	(@user_id, @role_name)
ON CONFLICT DO NOTHING
;

-- name: AssignRoleToServiceAccount :exec
INSERT INTO
	service_account_roles (service_account_id, role_name)
VALUES
	(@service_account_id, @role_name)
ON CONFLICT DO NOTHING
;

-- name: HasTeamAuthorization :one
SELECT
	(
		EXISTS (
			SELECT
				1
			FROM
				authorizations a
				INNER JOIN role_authorizations ra ON ra.authorization_name = a.name
				INNER JOIN user_roles ur ON ur.role_name = ra.role_name
			WHERE
				ur.user_id = @user_id
				AND a.name = @authorization_name
				AND (
					ur.target_team_slug = @team_slug::slug
					OR ur.target_team_slug IS NULL
				)
		)
		OR EXISTS (
			SELECT
				1
			FROM
				users
			WHERE
				id = @user_id
				AND admin = TRUE
		)
	)::BOOLEAN
;

-- name: HasGlobalAuthorization :one
SELECT
	(
		EXISTS (
			SELECT
				1
			FROM
				authorizations a
				INNER JOIN role_authorizations ra ON ra.authorization_name = a.name
				INNER JOIN user_roles ur ON ur.role_name = ra.role_name
			WHERE
				ur.user_id = @user_id
				AND a.name = @authorization_name
				AND ur.target_team_slug IS NULL
		)
		OR EXISTS (
			SELECT
				1
			FROM
				users
			WHERE
				id = @user_id
				AND admin = TRUE
		)
	)::BOOLEAN
;
