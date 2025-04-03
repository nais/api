-- name: TeamSlugsFromUserID :many
SELECT
	teams.slug
FROM
	user_roles
	JOIN teams ON teams.slug = user_roles.target_team_slug
WHERE
	user_roles.user_id = @user_id
ORDER BY
	teams.slug ASC
;
