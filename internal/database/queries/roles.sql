-- name: GetUserRoles :many
SELECT * FROM user_roles
WHERE user_id = @user_id
ORDER BY role_name ASC;

-- name: GetAllUserRoles :many
SELECT * FROM user_roles
ORDER BY role_name ASC;

-- name: AssignGlobalRoleToUser :exec
INSERT INTO user_roles (user_id, role_name)
VALUES (@user_id, @role_name) ON CONFLICT DO NOTHING;

-- name: AssignGlobalRoleToServiceAccount :exec
INSERT INTO service_account_roles (service_account_id, role_name)
VALUES (@service_account_id, @role_name) ON CONFLICT DO NOTHING;

-- name: AssignTeamRoleToUser :exec
INSERT INTO user_roles (user_id, role_name, target_team_slug)
VALUES (@user_id, @role_name, @target_team_slug::slug) ON CONFLICT DO NOTHING;

-- name: AssignTeamRoleToServiceAccount :exec
INSERT INTO service_account_roles (service_account_id, role_name, target_team_slug)
VALUES (@service_account_id, @role_name, @target_team_slug::slug) ON CONFLICT DO NOTHING;

-- name: RevokeGlobalUserRole :exec
DELETE FROM user_roles
WHERE user_id = @user_id
AND target_team_slug IS NULL
AND target_service_account_id IS NULL
AND role_name = @role_name;

-- name: RemoveAllServiceAccountRoles :exec
DELETE FROM service_account_roles
WHERE service_account_id = @service_account_id;

-- name: GetUsersWithGloballyAssignedRole :many
SELECT users.* FROM users
JOIN user_roles ON user_roles.user_id = users.id
WHERE user_roles.target_team_slug IS NULL
AND user_roles.target_service_account_id IS NULL
AND user_roles.role_name = @role_name
ORDER BY name, email ASC;
