-- name: ListIssuesForTeam :many
SELECT
	*
FROM
	issues
WHERE
	team = @team
ORDER BY
	id DESC
;
