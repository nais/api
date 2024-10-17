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
