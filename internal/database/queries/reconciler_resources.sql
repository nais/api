-- name: GetReconcilerResourcesForReconciler :many
SELECT *
FROM reconciler_resources
WHERE reconciler_name = @reconciler_name
ORDER BY team_slug, name ASC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: GetReconcilerResourcesForReconcilerAndTeam :many
SELECT *
FROM reconciler_resources
WHERE reconciler_name = @reconciler_name AND team_slug = @team_slug
ORDER BY team_slug, name ASC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: UpsertReconcilerResource :one
INSERT INTO reconciler_resources (
  reconciler_name,
  team_slug,
  name,
  value,
  metadata
) VALUES (
  @reconciler_name,
  @team_slug,
  @name,
  @value,
  COALESCE(@metadata, '{}'::jsonb)
)
ON CONFLICT (reconciler_name, team_slug, name) DO
UPDATE SET value = EXCLUDED.value, metadata = EXCLUDED.metadata
RETURNING *;
