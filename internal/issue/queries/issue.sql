-- name: ListIssuesForTeam :many
SELECT
	*
FROM
	issues
WHERE
	team = @team
ORDER BY
	severity,
	env,
	issue_type,
	resource_type,
	resource_name,
	id
;
