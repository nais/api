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
ON CONFLICT (reconciler_name, team_slug, name, value) DO
UPDATE SET metadata = EXCLUDED.metadata
RETURNING *;

-- name: GetReconcilerResourceByKey :many
SELECT *
FROM reconciler_resources
WHERE reconciler_name = @reconciler_name AND team_slug = @team_slug AND name = @name
ORDER BY value ASC LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: GetReconcilerResourceByKeyAndValue :one
SELECT *
FROM reconciler_resources
WHERE reconciler_name = @reconciler_name AND team_slug = @team_slug AND name = @name AND value = @value;

-- name: GetReconcilerResourceByKeyTotal :one
SELECT COUNT(*)
FROM reconciler_resources
WHERE reconciler_name = @reconciler_name AND team_slug = @team_slug AND name = @name;

-- name: DeleteAllReconcilerResources :exec
DELETE FROM reconciler_resources
WHERE reconciler_name = @reconciler_name AND team_slug = @team_slug;
