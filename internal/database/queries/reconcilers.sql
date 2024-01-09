-- name: GetReconcilers :many
SELECT * FROM reconcilers
ORDER BY run_order ASC;

-- name: GetEnabledReconcilers :many
SELECT * FROM reconcilers
WHERE enabled = true
ORDER BY run_order ASC;

-- name: GetReconciler :one
SELECT * FROM reconcilers
WHERE name = @name;

-- name: EnableReconciler :one
UPDATE reconcilers
SET enabled = true
WHERE name = @name
RETURNING *;

-- name: DisableReconciler :one
UPDATE reconcilers
SET enabled = false
WHERE name = @name
RETURNING *;

-- name: ResetReconcilerConfig :exec
UPDATE reconciler_config
SET value = NULL
WHERE reconciler = @reconciler_name;

-- name: ConfigureReconciler :exec
UPDATE reconciler_config
SET value = @value::TEXT
WHERE reconciler = @reconciler_name AND key = @key;

-- name: GetReconcilerConfig :many
SELECT
    rc.reconciler,
    rc.key,
    rc.display_name,
    rc.description,
    (rc.value IS NOT NULL)::BOOL AS configured,
    rc2.value,
    rc.secret
FROM reconciler_config rc
LEFT JOIN reconciler_config rc2 ON rc2.reconciler = rc.reconciler AND rc2.key = rc.key AND rc2.secret = false
WHERE rc.reconciler = @reconciler_name
ORDER BY rc.display_name ASC;

-- name: DangerousGetReconcilerConfigValues :many
SELECT key, value::TEXT
FROM reconciler_config
WHERE reconciler = @reconciler_name
ORDER BY key ASC;

-- name: AddReconcilerOptOut :exec
INSERT INTO reconciler_opt_outs (team_slug, user_id, reconciler_name)
VALUES (@team_slug, @user_id, @reconciler_name) ON CONFLICT DO NOTHING;

-- name: RemoveReconcilerOptOut :exec
DELETE FROM reconciler_opt_outs
WHERE team_slug = @team_slug AND user_id = @user_id AND reconciler_name = @reconciler_name;
