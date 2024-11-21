-- name: AssignTeamRoleToServiceAccount :exec
INSERT INTO
	service_account_roles (service_account_id, role_name, target_team_slug)
VALUES
	(
		@service_account_id,
		@role_name,
		@target_team_slug::slug
	)
ON CONFLICT DO NOTHING
;

-- name: AssignTeamRoleToUser :exec
INSERT INTO
	user_roles (user_id, role_name, target_team_slug)
VALUES
	(@user_id, @role_name, @target_team_slug::slug)
ON CONFLICT DO NOTHING
;

-- name: GetRolesForUsers :many
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
	user_id = ANY (@user_ids::UUID [])
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
			target_team_slug,
			'target_service_account_id',
			target_service_account_id
		)
	) AS roles
FROM
	service_account_roles
WHERE
	service_account_id = ANY (@service_account_ids::UUID [])
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

-- name: AssignGlobalRoleToServiceAccount :exec
INSERT INTO
	service_account_roles (service_account_id, role_name)
VALUES
	(@service_account_id, @role_name)
ON CONFLICT DO NOTHING
;
