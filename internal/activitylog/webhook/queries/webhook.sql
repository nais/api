-- name: CreateSubscription :one
INSERT INTO
	webhook_subscriptions (
		name,
		team_slug,
		source_team_slugs,
		url,
		secret,
		event_types
	)
VALUES
	(
		@name,
		@team_slug,
		@source_team_slugs::slug[],
		@url,
		@secret,
		@event_types
	)
RETURNING
	*
;

-- name: UpdateSubscription :one
UPDATE webhook_subscriptions
SET
	name = COALESCE(sqlc.narg(name), name),
	url = COALESCE(sqlc.narg(url), url),
	secret = COALESCE(sqlc.narg(secret), secret),
	event_types = COALESCE(sqlc.narg(event_types), event_types),
	source_team_slugs = COALESCE(
		sqlc.narg(source_team_slugs)::slug[],
		source_team_slugs
	),
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

-- name: ReserveTestDelivery :one
UPDATE webhook_subscriptions
SET
	last_test_at = NOW()
WHERE
	id = @id
	AND (
		last_test_at IS NULL
		OR last_test_at <= NOW() - INTERVAL '30 seconds'
	)
RETURNING
	*
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

-- name: GetSubscriptionForUpdate :one
SELECT
	*
FROM
	webhook_subscriptions
WHERE
	id = @id
FOR UPDATE
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

-- name: ListExistingSourceTeams :many
SELECT
	slug
FROM
	teams
WHERE
	slug = ANY (@slugs::slug[])
ORDER BY
	slug
;

-- name: RecordDeliveryResult :one
-- Serialize result bookkeeping for each subscription and report an auto-disable transition only once.
WITH
	previous AS (
		SELECT
			webhook_subscriptions.id,
			webhook_subscriptions.enabled
		FROM
			webhook_subscriptions
		WHERE
			webhook_subscriptions.id = @id
		FOR UPDATE
	)
UPDATE webhook_subscriptions AS subscription
SET
	consecutive_failures = CASE
		WHEN @success::BOOLEAN THEN 0
		ELSE subscription.consecutive_failures + 1
	END,
	enabled = CASE
		WHEN NOT @success::BOOLEAN
		AND subscription.consecutive_failures + 1 >= @disable_threshold::INT THEN FALSE
		ELSE subscription.enabled
	END,
	disabled_at = CASE
		WHEN previous.enabled
		AND NOT @success::BOOLEAN
		AND subscription.consecutive_failures + 1 >= @disable_threshold::INT THEN NOW()
		ELSE subscription.disabled_at
	END
FROM
	previous
WHERE
	subscription.id = previous.id
RETURNING
	COALESCE(
		previous.enabled
		AND NOT subscription.enabled,
		FALSE
	)::BOOLEAN AS auto_disabled
;

-- name: CreateDelivery :one
-- Allocate attempt numbers atomically, including after an expired claim is recovered.
WITH
	numbered_delivery AS (
		UPDATE webhook_event_deliveries
		SET
			attempt_count = webhook_event_deliveries.attempt_count + 1
		WHERE
			webhook_event_deliveries.id = sqlc.narg(webhook_event_delivery_id)
		RETURNING
			webhook_event_deliveries.attempt_count
	)
INSERT INTO
	webhook_deliveries (
		subscription_id,
		webhook_event_delivery_id,
		event_type,
		request_body,
		request_headers,
		response_status,
		response_body,
		response_headers,
		response_body_truncated,
		error_message,
		duration_ms,
		success,
		attempt_count
	)
VALUES
	(
		@subscription_id,
		sqlc.narg(webhook_event_delivery_id),
		@event_type,
		@request_body,
		@request_headers,
		@response_status,
		@response_body,
		@response_headers,
		@response_body_truncated,
		sqlc.narg(error_message),
		@duration_ms,
		@success,
		COALESCE(
			(
				SELECT
					numbered_delivery.attempt_count
				FROM
					numbered_delivery
			),
			1
		)
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

-- name: ClaimOutboxEventsForFanout :many
-- Claims outbox events pending fan-out. FOR UPDATE SKIP LOCKED lets multiple dispatcher
-- instances claim batches concurrently without claiming the same row. Rows are marked
-- completed by the caller after fan-out succeeds, not by this query.
SELECT
	sqlc.embed(webhook_events),
	sqlc.embed(activity_log_entries)
FROM
	webhook_events
	JOIN activity_log_entries ON webhook_events.activity_log_entries_id = activity_log_entries.id
WHERE
	webhook_events.status = 'pending'
ORDER BY
	webhook_events.created_at ASC
LIMIT
	sqlc.arg('batch_size')
FOR UPDATE OF
	webhook_events SKIP LOCKED
;

-- name: MarkOutboxEventsCompleted :exec
UPDATE webhook_events
SET
	status = 'completed'
WHERE
	id = ANY (@ids::UUID[])
;

-- name: CreateEventDelivery :exec
INSERT INTO
	webhook_event_deliveries (webhook_event_id, subscription_id)
VALUES
	(@webhook_event_id, @subscription_id)
ON CONFLICT (webhook_event_id, subscription_id) DO NOTHING
;

-- name: ClaimPendingDeliveries :many
-- Claims per-(event, subscription) delivery rows for processing, using the same
-- FOR UPDATE SKIP LOCKED pattern as ClaimOutboxEventsForFanout.
WITH
	claimed_deliveries AS (
		UPDATE webhook_event_deliveries
		SET
			status = 'processing',
			claim_token = GEN_RANDOM_UUID(),
			lease_expires_at = NOW() + INTERVAL '1 minute'
		WHERE
			id IN (
				SELECT
					id
				FROM
					webhook_event_deliveries
				WHERE
					(
						(
							status = 'pending'
							AND run_at <= NOW()
						)
						OR (
							status = 'processing'
							AND lease_expires_at <= NOW()
						)
					)
					AND EXISTS (
						SELECT
							1
						FROM
							webhook_subscriptions
						WHERE
							webhook_subscriptions.id = webhook_event_deliveries.subscription_id
							AND webhook_subscriptions.enabled
					)
				ORDER BY
					run_at ASC
				LIMIT
					sqlc.arg('batch_size')
				FOR UPDATE
					SKIP LOCKED
			)
		RETURNING
			*
	)
SELECT
	sqlc.embed(webhook_event_deliveries),
	sqlc.embed(webhook_subscriptions),
	sqlc.embed(activity_log_entries)
FROM
	claimed_deliveries webhook_event_deliveries
	JOIN webhook_subscriptions ON webhook_event_deliveries.subscription_id = webhook_subscriptions.id
	JOIN webhook_events ON webhook_event_deliveries.webhook_event_id = webhook_events.id
	JOIN activity_log_entries ON webhook_events.activity_log_entries_id = activity_log_entries.id
;

-- name: RequeueDelivery :exec
WITH
	requeued AS (
		UPDATE webhook_event_deliveries
		SET
			status = 'pending',
			lease_expires_at = NULL,
			retry_count = @retry_count,
			run_at = @run_at
		WHERE
			webhook_event_deliveries.id = @id
			AND webhook_event_deliveries.claim_token = @claim_token
			AND webhook_event_deliveries.status = 'processing'
		RETURNING
			webhook_event_deliveries.id,
			webhook_event_deliveries.run_at
	)
UPDATE webhook_deliveries
SET
	next_retry_at = requeued.run_at
FROM
	requeued
WHERE
	webhook_deliveries.id = @attempt_id
	AND webhook_deliveries.webhook_event_delivery_id = requeued.id
;

-- name: MarkDeliveryFailed :exec
UPDATE webhook_event_deliveries
SET
	status = 'failed',
	lease_expires_at = NULL
WHERE
	id = @id
	AND claim_token = @claim_token
	AND status = 'processing'
;

-- name: MarkDeliveryCompleted :exec
UPDATE webhook_event_deliveries
SET
	status = 'completed',
	lease_expires_at = NULL
WHERE
	id = @id
	AND claim_token = @claim_token
	AND status = 'processing'
;

-- name: PruneOldOutboxEvents :exec
-- Prunes outbox events whose deliveries have all reached a terminal state (a delete
-- cascades to webhook_event_deliveries, so pending and processing rows must be excluded).
DELETE FROM webhook_events
WHERE
	webhook_events.created_at < @before
	AND webhook_events.status = 'completed'
	AND NOT EXISTS (
		SELECT
			1
		FROM
			webhook_event_deliveries
		WHERE
			webhook_event_deliveries.webhook_event_id = webhook_events.id
			AND webhook_event_deliveries.status IN ('pending', 'processing')
	)
;

-- name: PruneOldEventDeliveries :exec
DELETE FROM webhook_event_deliveries
WHERE
	created_at < @before
	AND status IN ('completed', 'failed')
;

-- name: GetQueueSizeByStatus :many
SELECT
	status,
	COUNT(*) AS count
FROM
	webhook_event_deliveries
GROUP BY
	status
ORDER BY
	status
;
