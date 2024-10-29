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

-- name: GetUserRolesForUsers :many
SELECT
	user_id,
	JSON_AGG(
		JSON_BUILD_OBJECT(
			'role_name',
			role_name,
			'target_team_slug',
			target_team_slug
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
