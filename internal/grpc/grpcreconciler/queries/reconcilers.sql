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

-- name: SetLastSuccessfulSyncForTeam :exec
UPDATE teams
SET
	last_successful_sync = NOW()
WHERE
	teams.slug = @slug
;

-- name: Upsert :one
INSERT INTO
	reconcilers (
		name,
		display_name,
		description,
		member_aware,
		enabled
	)
VALUES
	(
		@name,
		@display_name,
		@description,
		@member_aware,
		@enabled_if_new
	)
ON CONFLICT (name) DO
UPDATE
SET
	display_name = EXCLUDED.display_name,
	description = EXCLUDED.description,
	member_aware = EXCLUDED.member_aware
RETURNING
	*
;

-- name: Get :one
SELECT
	*
FROM
	reconcilers
WHERE
	name = @name
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

-- name: UpsertConfig :exec
INSERT INTO
	reconciler_config (
		reconciler,
		key,
		display_name,
		description,
		secret
	)
VALUES
	(
		@reconciler,
		@key,
		@display_name,
		@description,
		@secret
	)
ON CONFLICT (reconciler, key) DO
UPDATE
SET
	display_name = EXCLUDED.display_name,
	description = EXCLUDED.description,
	secret = EXCLUDED.secret
;

-- name: DeleteConfig :exec
DELETE FROM reconciler_config
WHERE
	reconciler = @reconciler
	AND key = ANY (@keys::TEXT[])
;
