-- name: GetReconcilers :many
SELECT * FROM reconcilers
ORDER BY display_name ASC LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: GetReconcilersCount :one
SELECT COUNT(*) as total FROM reconcilers;

-- name: GetEnabledReconcilers :many
SELECT * FROM reconcilers
WHERE enabled = true
ORDER BY display_name ASC;

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
LEFT JOIN reconciler_config rc2 ON rc2.reconciler = rc.reconciler AND rc2.key = rc.key AND (rc2.secret = false OR @include_secret::bool = true)
WHERE rc.reconciler = @reconciler_name
ORDER BY rc.display_name ASC;

-- name: UpsertReconciler :one
INSERT INTO reconcilers (name, display_name, description, member_aware, enabled)
VALUES (@name, @display_name, @description, @member_aware, @enabled_if_new)
ON CONFLICT (name) DO UPDATE
SET display_name = EXCLUDED.display_name, description = EXCLUDED.description, member_aware = EXCLUDED.member_aware
RETURNING *;

-- name: UpsertReconcilerConfig :exec
INSERT INTO reconciler_config (reconciler, key, display_name, description, secret)
VALUES (@reconciler, @key, @display_name, @description, @secret)
ON CONFLICT (reconciler, key) DO UPDATE
SET display_name = EXCLUDED.display_name, description = EXCLUDED.description, secret = EXCLUDED.secret;

-- name: DeleteReconcilerConfig :exec
DELETE FROM reconciler_config
WHERE reconciler = @reconciler AND key = ANY(@keys::TEXT[]);
