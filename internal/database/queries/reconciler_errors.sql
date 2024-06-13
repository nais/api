-- name: ClearReconcilerErrorsForTeam :exec
DELETE FROM reconciler_errors
WHERE team_slug = @team_slug AND reconciler = @reconciler;

-- name: SetReconcilerErrorForTeam :exec
INSERT INTO reconciler_errors (correlation_id, team_slug, reconciler, error_message)
VALUES (@correlation_id, @team_slug, @reconciler, @error_message)
ON CONFLICT(team_slug, reconciler) DO
    UPDATE SET correlation_id = EXCLUDED.correlation_id, created_at = NOW(), error_message = EXCLUDED.error_message;

-- name: GetTeamReconcilerErrors :many
SELECT * FROM reconciler_errors
WHERE team_slug = @team_slug
ORDER BY created_at DESC;

-- name: GetReconcilerErrors :many
SELECT sqlc.embed(reconciler_errors), sqlc.embed(reconcilers) FROM reconciler_errors
JOIN reconcilers ON reconcilers.name = reconciler_errors.reconciler
WHERE
    reconcilers.enabled = true
    AND reconciler_errors.reconciler = @reconciler
ORDER BY
    reconciler_errors.created_at DESC
LIMIT sqlc.arg('limit')
OFFSET sqlc.arg('offset');

-- name: GetReconcilerErrorsCount :one
SELECT COUNT(*) FROM reconciler_errors;
