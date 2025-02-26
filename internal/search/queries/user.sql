-- SELECT
-- 	sqlc.embed(users),
-- 	sqlc.embed(user_roles)
-- FROM
-- 	user_roles
-- 	JOIN teams ON teams.slug = user_roles.target_team_slug
-- 	JOIN users ON users.id = user_roles.user_id
-- WHERE
-- 	user_roles.user_id = @user_id
-- ORDER BY
-- 	CASE
-- 		WHEN @order_by::TEXT = 'slug:asc' THEN teams.slug
-- 	END ASC,
-- 	CASE
-- 		WHEN @order_by::TEXT = 'slug:desc' THEN teams.slug
-- 	END DESC,
-- 	teams.slug ASC
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
