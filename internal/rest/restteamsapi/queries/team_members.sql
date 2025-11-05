-- name: ListMembers :many
SELECT
	users.external_id
FROM
	users,
	user_roles
WHERE
	users.id = user_roles.user_id
	AND user_roles.target_team_slug = @team_slug::slug
ORDER BY
	users.email ASC
;

-- name: TeamExists :one
SELECT
	EXISTS (
		SELECT
			slug
		FROM
			teams
		WHERE
			slug = @slug
	)
;
