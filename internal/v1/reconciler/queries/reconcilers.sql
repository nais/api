-- name: List :many
SELECT
	*
FROM
	reconcilers
ORDER BY
	display_name ASC
LIMIT
	sqlc.arg('limit')
OFFSET
	sqlc.arg('offset')
;

-- name: Count :one
SELECT
	COUNT(*) AS total
FROM
	reconcilers
;

-- name: ListEnabledReconcilers :many
SELECT
	*
FROM
	reconcilers
WHERE
	enabled = TRUE
ORDER BY
	display_name ASC
;

-- name: Get :one
SELECT
	*
FROM
	reconcilers
WHERE
	name = @name
;

-- name: Enable :one
UPDATE reconcilers
SET
	enabled = TRUE
WHERE
	name = @name
RETURNING
	*
;

-- name: Disable :one
UPDATE reconcilers
SET
	enabled = FALSE
WHERE
	name = @name
RETURNING
	*
;

-- name: ListByNames :many
SELECT
	*
FROM
	reconcilers
WHERE
	name = ANY (@names::TEXT[])
ORDER BY
	name ASC
;

-- name: GetConfig :many
SELECT
	rc.reconciler,
	rc.key,
	rc.display_name,
	rc.description,
	(rc.value IS NOT NULL)::BOOL AS configured,
	rc2.value,
	rc.secret
FROM
	reconciler_config rc
	LEFT JOIN reconciler_config rc2 ON rc2.reconciler = rc.reconciler
	AND rc2.key = rc.key
	AND (
		rc2.secret = FALSE
		OR @include_secret::BOOL = TRUE
	)
WHERE
	rc.reconciler = @reconciler_name
ORDER BY
	rc.display_name ASC
;

-- name: Configure :exec
UPDATE reconciler_config
SET
	value = @value::TEXT
WHERE
	reconciler = @reconciler_name
	AND key = @key
;

-- name: GetErrors :many
SELECT
	reconciler_errors.*
FROM
	reconciler_errors
	JOIN reconcilers ON reconcilers.name = reconciler_errors.reconciler
WHERE
	reconcilers.enabled = TRUE
	AND reconciler_errors.reconciler = @reconciler
ORDER BY
	reconciler_errors.created_at DESC
LIMIT
	sqlc.arg('limit')
OFFSET
	sqlc.arg('offset')
;

-- name: GetErrorsCount :one
SELECT
	COUNT(*)
FROM
	reconciler_errors
	JOIN reconcilers ON reconcilers.name = reconciler_errors.reconciler
WHERE
	reconcilers.enabled = TRUE
	AND reconciler_errors.reconciler = @reconciler
;
