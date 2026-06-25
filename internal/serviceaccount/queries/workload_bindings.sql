-- name: CreateBinding :one
INSERT INTO
	service_account_workload_bindings (
		service_account_id,
		environment,
		team_slug,
		workload_name
	)
VALUES
	(
		@service_account_id,
		@environment,
		@team_slug,
		@workload_name
	)
RETURNING
	*
;

-- name: DeleteBinding :exec
DELETE FROM service_account_workload_bindings
WHERE
	id = @id
;

-- name: GetBindingByID :one
SELECT
	*
FROM
	service_account_workload_bindings
WHERE
	id = @id
;

-- name: GetBindingByWorkload :one
SELECT
	*
FROM
	service_account_workload_bindings
WHERE
	environment = @environment
	AND team_slug = @team_slug
	AND workload_name = @workload_name
;

-- name: ListBindingsForServiceAccount :many
SELECT
	sqlc.embed(service_account_workload_bindings),
	COUNT(*) OVER () AS total_count
FROM
	service_account_workload_bindings
WHERE
	service_account_id = @service_account_id
ORDER BY
	environment,
	team_slug,
	workload_name
LIMIT
	sqlc.arg('limit')
OFFSET
	sqlc.arg('offset')
;

-- name: GetBindingsByIDs :many
SELECT
	*
FROM
	service_account_workload_bindings
WHERE
	id = ANY (@ids::UUID[])
ORDER BY
	created_at
;

-- name: UpdateBindingLastUsedAt :exec
UPDATE service_account_workload_bindings
SET
	last_used_at = CLOCK_TIMESTAMP()
WHERE
	id = @id
;
