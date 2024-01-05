-- name: GetReconcilers :many
SELECT * FROM reconcilers
ORDER BY run_order ASC;

-- name: GetEnabledReconcilers :many
SELECT * FROM reconcilers
WHERE enabled = true
ORDER BY run_order ASC;

-- name: GetReconciler :one
SELECT * FROM reconcilers
WHERE name = $1;

-- name: EnableReconciler :one
UPDATE reconcilers
SET enabled = true
WHERE name = $1
RETURNING *;

-- name: DisableReconciler :one
UPDATE reconcilers
SET enabled = false
WHERE name = $1
RETURNING *;

-- name: ResetReconcilerConfig :exec
UPDATE reconciler_config
SET value = NULL
WHERE reconciler = $1;

-- name: ConfigureReconciler :exec
UPDATE reconciler_config
SET value = sqlc.arg(value)::TEXT
WHERE reconciler = $1 AND key = $2;

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
WHERE rc.reconciler = $1
ORDER BY rc.display_name ASC;

-- name: DangerousGetReconcilerConfigValues :many
SELECT key, value::TEXT
FROM reconciler_config
WHERE reconciler = $1;

-- name: AddReconcilerOptOut :exec
INSERT INTO reconciler_opt_outs (team_slug, user_id, reconciler_name)
VALUES ($1, $2, $3) ON CONFLICT DO NOTHING;

-- name: RemoveReconcilerOptOut :exec
DELETE FROM reconciler_opt_outs
WHERE team_slug = $1 AND user_id = $2 AND reconciler_name = $3;