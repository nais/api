-- name: CreateSubscription :one
INSERT INTO
	webhook_subscriptions (team_slug, url, secret, event_types, created_by)
VALUES
	(
		@team_slug,
		@url,
		@secret,
		@event_types,
		@created_by
	)
RETURNING
	*
;

-- name: UpdateSubscription :one
UPDATE webhook_subscriptions
SET
	url = COALESCE(sqlc.narg(url), url),
	secret = COALESCE(sqlc.narg(secret), secret),
	event_types = COALESCE(sqlc.narg(event_types), event_types),
	enabled = COALESCE(sqlc.narg(enabled), enabled)
WHERE
	id = @id
RETURNING
	*
;

-- name: DeleteSubscription :exec
DELETE FROM webhook_subscriptions
WHERE
	id = @id
;

-- name: GetSubscription :one
SELECT
	*
FROM
	webhook_subscriptions
WHERE
	id = @id
;

-- name: ListSubscriptionsByIDs :many
SELECT
	*
FROM
	webhook_subscriptions
WHERE
	id = ANY (@ids::UUID[])
ORDER BY
	created_at DESC
;

-- name: ListSubscriptionsForTeam :many
SELECT
	sqlc.embed(webhook_subscriptions),
	COUNT(*) OVER () AS total_count
FROM
	webhook_subscriptions
WHERE
	team_slug = @team_slug
ORDER BY
	created_at DESC
LIMIT
	sqlc.arg('limit')
OFFSET
	sqlc.arg('offset')
;

-- name: ListGlobalSubscriptions :many
SELECT
	sqlc.embed(webhook_subscriptions),
	COUNT(*) OVER () AS total_count
FROM
	webhook_subscriptions
WHERE
	team_slug IS NULL
ORDER BY
	created_at DESC
LIMIT
	sqlc.arg('limit')
OFFSET
	sqlc.arg('offset')
;

-- name: ListEnabledSubscriptions :many
SELECT
	*
FROM
	webhook_subscriptions
WHERE
	enabled = TRUE
ORDER BY
	created_at DESC
;

-- name: IncrementConsecutiveFailures :one
UPDATE webhook_subscriptions
SET
	consecutive_failures = consecutive_failures + 1
WHERE
	id = @id
RETURNING
	*
;

-- name: ResetConsecutiveFailures :exec
UPDATE webhook_subscriptions
SET
	consecutive_failures = 0
WHERE
	id = @id
;

-- name: DisableSubscription :exec
UPDATE webhook_subscriptions
SET
	enabled = FALSE,
	disabled_at = NOW()
WHERE
	id = @id
;

-- name: CreateDelivery :one
INSERT INTO
	webhook_deliveries (
		subscription_id,
		event_type,
		request_body,
		response_status,
		response_body,
		duration_ms,
		success
	)
VALUES
	(
		@subscription_id,
		@event_type,
		@request_body,
		@response_status,
		@response_body,
		@duration_ms,
		@success
	)
RETURNING
	*
;

-- name: GetDelivery :one
SELECT
	*
FROM
	webhook_deliveries
WHERE
	id = @id
;

-- name: ListDeliveriesByIDs :many
SELECT
	*
FROM
	webhook_deliveries
WHERE
	id = ANY (@ids::UUID[])
ORDER BY
	created_at DESC
;

-- name: ListDeliveriesForSubscription :many
SELECT
	sqlc.embed(webhook_deliveries),
	COUNT(*) OVER () AS total_count
FROM
	webhook_deliveries
WHERE
	subscription_id = @subscription_id
ORDER BY
	created_at DESC
LIMIT
	sqlc.arg('limit')
OFFSET
	sqlc.arg('offset')
;

-- name: PruneDeliveries :exec
DELETE FROM webhook_deliveries
WHERE
	created_at < @before
;

-- name: ClaimPendingEvents :many
WITH
	updated_events AS (
		UPDATE webhook_events
		SET
			status = 'completed'
		WHERE
			id IN (
				SELECT
					id
				FROM
					webhook_events
				WHERE
					status = 'pending'
					AND run_at <= NOW()
				ORDER BY
					run_at ASC
				LIMIT
					@batch_size
				FOR UPDATE
					SKIP LOCKED
			)
		RETURNING
			*
	)
SELECT
	sqlc.embed(webhook_events),
	sqlc.embed(activity_log_entries)
FROM
	updated_events webhook_events
	JOIN activity_log_entries ON webhook_events.activity_log_entries_id = activity_log_entries.id
;

-- name: RequeueEvent :exec
UPDATE webhook_events
SET
	status = 'pending',
	retry_count = @retry_count,
	run_at = @run_at
WHERE
	id = @id
;

-- name: MarkEventFailed :exec
UPDATE webhook_events
SET
	status = 'failed'
WHERE
	id = @id
;

-- name: PruneOldEvents :exec
DELETE FROM webhook_events
WHERE
	created_at < @before
	AND status IN ('completed', 'failed')
;

-- name: GetQueueSizeByStatus :many
SELECT
	status,
	COUNT(*) AS count
FROM
	webhook_events
GROUP BY
	status
ORDER BY
	status
;
