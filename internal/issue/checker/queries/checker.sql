-- name: BatchInsertIssues :batchexec
INSERT INTO
	issues (
		issue_type,
		resource_name,
		resource_type,
		team,
		env,
		severity,
		message,
		issue_details
	)
VALUES
	(
		@issue_type,
		@resource_name,
		@resource_type,
		@team,
		@env,
		@severity,
		@message,
		@issue_details
	)
;

-- name: DeleteIssues :exec
DELETE FROM issues
;
