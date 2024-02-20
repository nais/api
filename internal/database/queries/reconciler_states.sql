-- name: GetReconcilerState :many
SELECT sqlc.embed(teams), sqlc.embed(reconciler_states)
FROM reconciler_states
JOIN teams ON teams.slug = reconciler_states.team_slug
WHERE reconciler_name = @reconciler_name
ORDER BY team_slug ASC;

-- name: GetReconcilerStateForTeam :one
SELECT *
FROM reconciler_states
WHERE reconciler_name = @reconciler_name AND team_slug = @team_slug;

-- name: UpsertReconcilerState :one
INSERT INTO reconciler_states (
    reconciler_name,
    team_slug,
    value
) VALUES (
    @reconciler_name,
    @team_slug,
    @value
)
ON CONFLICT DO
UPDATE SET value = EXCLUDED.value
RETURNING *;

-- name: DeleteReconcilerStateForTeam :exec
DELETE FROM reconciler_states
WHERE reconciler_name = @reconciler_name AND team_slug = @team_slug;
