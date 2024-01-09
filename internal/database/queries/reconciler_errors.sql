-- name: ClearReconcilerErrorsForTeam :exec
DELETE FROM reconciler_errors
WHERE team_slug = @team_slug AND reconciler = @reconciler;

-- name: SetReconcilerErrorForTeam :exec
INSERT INTO reconciler_errors (correlation_id, team_slug, reconciler, error_message)
VALUES (@correlation_id, @team_slug, @reconciler, @error_message)
ON CONFLICT(team_slug, reconciler) DO
    UPDATE SET correlation_id = @correlation_id, created_at = NOW(), error_message = @error_message;

-- name: GetTeamReconcilerErrors :many
SELECT * FROM reconciler_errors
WHERE team_slug = @team_slug
ORDER BY created_at DESC;
