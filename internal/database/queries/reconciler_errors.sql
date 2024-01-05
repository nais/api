-- name: ClearReconcilerErrorsForTeam :exec
DELETE FROM reconciler_errors
WHERE team_slug = $1 AND reconciler = $2;

-- name: SetReconcilerErrorForTeam :exec
INSERT INTO reconciler_errors (correlation_id, team_slug, reconciler, error_message)
VALUES ($1, $2, $3, $4)
ON CONFLICT(team_slug, reconciler) DO
    UPDATE SET correlation_id = $1, created_at = NOW(), error_message = $4;

-- name: GetTeamReconcilerErrors :many
SELECT * FROM reconciler_errors
WHERE team_slug = $1
ORDER BY created_at DESC;