-- +goose Up
CREATE TABLE webhook_subscriptions (
	id UUID PRIMARY KEY DEFAULT GEN_RANDOM_UUID(),
	team_slug slug REFERENCES teams (slug) ON DELETE CASCADE,
	url TEXT NOT NULL,
	secret TEXT NOT NULL,
	event_types TEXT[] NOT NULL,
	enabled BOOLEAN NOT NULL DEFAULT TRUE,
	consecutive_failures INT NOT NULL DEFAULT 0,
	disabled_at TIMESTAMPTZ,
	created_by TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)
;

CREATE TRIGGER webhook_subscriptions_updated_at
BEFORE UPDATE ON webhook_subscriptions FOR EACH ROW
EXECUTE FUNCTION set_updated_at ()
;

CREATE INDEX idx_webhook_subscriptions_team ON webhook_subscriptions (team_slug)
;

CREATE INDEX idx_webhook_subscriptions_global ON webhook_subscriptions (id)
WHERE
	team_slug IS NULL
;

CREATE INDEX idx_webhook_subscriptions_enabled ON webhook_subscriptions (enabled)
WHERE
	enabled = TRUE
;

CREATE TABLE webhook_deliveries (
	id UUID PRIMARY KEY DEFAULT GEN_RANDOM_UUID(),
	subscription_id UUID NOT NULL REFERENCES webhook_subscriptions (id) ON DELETE CASCADE,
	event_type TEXT NOT NULL,
	request_body JSONB NOT NULL,
	response_status INT,
	response_body TEXT,
	duration_ms INT NOT NULL,
	success BOOLEAN NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)
;

CREATE INDEX idx_webhook_deliveries_subscription ON webhook_deliveries (subscription_id, created_at DESC)
;

-- Outbox table for durable webhook event processing.
-- Rows are inserted by a trigger on activity_log_entries and consumed by the dispatcher.
CREATE TYPE webhook_event_status AS ENUM('pending', 'completed', 'failed')
;

CREATE TABLE webhook_events (
	id UUID PRIMARY KEY DEFAULT GEN_RANDOM_UUID(),
	activity_log_entries_id UUID NOT NULL REFERENCES activity_log_entries (id) ON DELETE CASCADE,
	status webhook_event_status NOT NULL DEFAULT 'pending',
	retry_count INT NOT NULL DEFAULT 0,
	run_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)
;

CREATE INDEX idx_webhook_events_pending ON webhook_events (run_at ASC)
WHERE
	status = 'pending'
;

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION webhook_events_notify () RETURNS trigger AS $$
BEGIN
	INSERT INTO webhook_events (activity_log_entries_id)
	VALUES (
		NEW.id
	);

	PERFORM pg_notify('api_notify', jsonb_build_object('table', 'webhook_events', 'op', 'INSERT', 'data', '{}'::jsonb)::text);
	RETURN NULL;
END;
$$ LANGUAGE plpgsql
;

-- +goose StatementEnd
CREATE TRIGGER activity_log_webhook_notify
AFTER INSERT ON activity_log_entries FOR EACH ROW
EXECUTE FUNCTION webhook_events_notify ()
;

INSERT INTO
	authorizations (name, description)
VALUES
	(
		'webhooks:create',
		'Permission to create webhook subscriptions.'
	),
	(
		'webhooks:update',
		'Permission to update webhook subscriptions.'
	),
	(
		'webhooks:delete',
		'Permission to delete webhook subscriptions.'
	)
;

INSERT INTO
	role_authorizations (role_name, authorization_name)
VALUES
	('Team owner', 'webhooks:create'),
	('Team owner', 'webhooks:update'),
	('Team owner', 'webhooks:delete')
;

-- +goose Down
DROP TRIGGER IF EXISTS activity_log_webhook_notify ON activity_log_entries
;

DROP FUNCTION IF EXISTS webhook_events_notify
;

DROP TABLE IF EXISTS webhook_events
;

DROP TYPE IF EXISTS webhook_event_status
;

DELETE FROM role_authorizations
WHERE
	authorization_name IN (
		'webhooks:create',
		'webhooks:update',
		'webhooks:delete'
	)
;

DELETE FROM authorizations
WHERE
	name IN (
		'webhooks:create',
		'webhooks:update',
		'webhooks:delete'
	)
;

DROP TABLE IF EXISTS webhook_deliveries
;

DROP TABLE IF EXISTS webhook_subscriptions
;
