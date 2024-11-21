-- name: UpsertState :one
INSERT INTO
	reconciler_states (reconciler_name, team_slug, value)
VALUES
	(@reconciler_name, @team_slug, @value)
ON CONFLICT (reconciler_name, team_slug) DO
UPDATE
SET
	value = EXCLUDED.value
RETURNING
	*
;

-- name: GetStateForTeam :one
SELECT
	*
FROM
	reconciler_states
WHERE
	reconciler_name = @reconciler_name
	AND team_slug = @team_slug
;

-- name: DeleteStateForTeam :exec
DELETE FROM reconciler_states
WHERE
	reconciler_name = @reconciler_name
	AND team_slug = @team_slug
;
