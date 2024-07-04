-- GetTeamMembers returns a slice of team members of a non-deleted team.
-- name: ListMembers :many
SELECT sqlc.embed(users), sqlc.embed(user_roles)
FROM user_roles
JOIN teams ON teams.slug = user_roles.target_team_slug
JOIN users ON users.id = user_roles.user_id
WHERE user_roles.target_team_slug = @team_slug::slug
ORDER BY
    CASE WHEN @order_by::TEXT = 'name:asc' THEN users.name END ASC,
    CASE WHEN @order_by::TEXT = 'name:desc' THEN users.name END DESC,
    CASE WHEN @order_by::TEXT = 'email:asc' THEN users.email END ASC,
    CASE WHEN @order_by::TEXT = 'email:desc' THEN users.email END DESC,
    CASE WHEN @order_by::TEXT = 'role:asc' THEN user_roles.role_name END ASC,
    CASE WHEN @order_by::TEXT = 'role:desc' THEN user_roles.role_name END DESC,
    users.name,
    users.email ASC
LIMIT sqlc.arg('limit')
OFFSET sqlc.arg('offset');

-- GetTeamMembersCount returns the total number of team members of a non-deleted team.
-- name: CountMembers :one
SELECT COUNT(user_roles.*)
FROM user_roles
JOIN teams ON teams.slug = user_roles.target_team_slug
WHERE user_roles.target_team_slug = @team_slug;