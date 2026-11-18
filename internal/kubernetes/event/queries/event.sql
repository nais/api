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
	k8s_events AS e
WHERE
	e.involved_namespace = @namespace
	AND e.environment_name = @environment_name
	AND e.reason = ANY (@reasons::TEXT[])
	AND (
		(
			e.involved_kind = ANY (@kinds::TEXT[])
			AND e.involved_name = @name
		)
		OR (
			e.data ->> 'workloadKind' = @workload_kind::TEXT
			AND e.data ->> 'workloadName' = @name
		)
	)
	AND (
		sqlc.narg('from')::TIMESTAMPTZ IS NULL
		OR e.triggered_at >= sqlc.narg('from')
	)
	AND (
		sqlc.narg('to')::TIMESTAMPTZ IS NULL
		OR e.triggered_at < sqlc.narg('to')
	)
	-- Repeated occurrences of aggregated reasons are stored as separate rows. Only the latest row within the time range is returned.
	AND (
		NOT e.reason = ANY (@aggregated_reasons::TEXT[])
		OR NOT EXISTS (
			SELECT
				1
			FROM
				k8s_events AS newer
			WHERE
				newer.uid = e.uid
				AND (newer.triggered_at, newer.id) > (e.triggered_at, e.id)
				AND (
					sqlc.narg('to')::TIMESTAMPTZ IS NULL
					OR newer.triggered_at < sqlc.narg('to')
				)
		)
	)
ORDER BY
	e.triggered_at DESC,
	e.id DESC
LIMIT
	sqlc.arg('limit')
OFFSET
	sqlc.arg('offset')
;

-- name: CountForWorkload :one
SELECT
	COUNT(*)
FROM
	k8s_events AS e
WHERE
	e.involved_namespace = @namespace
	AND e.environment_name = @environment_name
	AND e.reason = ANY (@reasons::TEXT[])
	AND (
		(
			e.involved_kind = ANY (@kinds::TEXT[])
			AND e.involved_name = @name
		)
		OR (
			e.data ->> 'workloadKind' = @workload_kind::TEXT
			AND e.data ->> 'workloadName' = @name
		)
	)
	AND (
		sqlc.narg('from')::TIMESTAMPTZ IS NULL
		OR e.triggered_at >= sqlc.narg('from')
	)
	AND (
		sqlc.narg('to')::TIMESTAMPTZ IS NULL
		OR e.triggered_at < sqlc.narg('to')
	)
	AND (
		NOT e.reason = ANY (@aggregated_reasons::TEXT[])
		OR NOT EXISTS (
			SELECT
				1
			FROM
				k8s_events AS newer
			WHERE
				newer.uid = e.uid
				AND (newer.triggered_at, newer.id) > (e.triggered_at, e.id)
				AND (
					sqlc.narg('to')::TIMESTAMPTZ IS NULL
					OR newer.triggered_at < sqlc.narg('to')
				)
		)
	)
;

-- name: ListByIDs :many
SELECT
	*
FROM
	k8s_events
WHERE
	id = ANY (@ids::UUID[])
	AND reason = ANY (@reasons::TEXT[])
ORDER BY
	triggered_at DESC,
	id DESC
;

-- name: DeleteOlderThan :execresult
DELETE FROM k8s_events
WHERE
	triggered_at < @before
;
