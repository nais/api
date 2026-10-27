-- name: Upsert :batchexec
INSERT INTO
	k8s_events (
		uid,
		environment_name,
		involved_kind,
		involved_name,
		involved_namespace,
		data,
		reason,
		triggered_at
	)
VALUES
	(
		@uid,
		@environment_name,
		@involved_kind,
		@involved_name,
		@involved_namespace,
		@data,
		@reason,
		@triggered_at
	)
ON CONFLICT (uid, triggered_at) DO NOTHING
;

-- name: ListForWorkload :many
SELECT
	*
FROM
	k8s_events
WHERE
	involved_namespace = @namespace
	AND environment_name = @environment_name
	AND reason IN ('SuccessfulRescale', 'Killing')
	AND (
		(
			involved_kind = ANY (@kinds::TEXT[])
			AND involved_name = @name
		)
		OR (
			data ->> 'workloadKind' = @workload_kind::TEXT
			AND data ->> 'workloadName' = @name
		)
	)
	AND (
		sqlc.narg('from')::TIMESTAMPTZ IS NULL
		OR triggered_at >= sqlc.narg('from')
	)
	AND (
		sqlc.narg('to')::TIMESTAMPTZ IS NULL
		OR triggered_at < sqlc.narg('to')
	)
ORDER BY
	triggered_at DESC,
	id DESC
LIMIT
	sqlc.arg('limit')
OFFSET
	sqlc.arg('offset')
;

-- name: CountForWorkload :one
SELECT
	COUNT(*)
FROM
	k8s_events
WHERE
	involved_namespace = @namespace
	AND environment_name = @environment_name
	AND reason IN ('SuccessfulRescale', 'Killing')
	AND (
		(
			involved_kind = ANY (@kinds::TEXT[])
			AND involved_name = @name
		)
		OR (
			data ->> 'workloadKind' = @workload_kind::TEXT
			AND data ->> 'workloadName' = @name
		)
	)
	AND (
		sqlc.narg('from')::TIMESTAMPTZ IS NULL
		OR triggered_at >= sqlc.narg('from')
	)
	AND (
		sqlc.narg('to')::TIMESTAMPTZ IS NULL
		OR triggered_at < sqlc.narg('to')
	)
;

-- name: ListByIDs :many
SELECT
	*
FROM
	k8s_events
WHERE
	id = ANY (@ids::UUID[])
	AND reason IN ('SuccessfulRescale', 'Killing')
ORDER BY
	triggered_at DESC,
	id DESC
;
