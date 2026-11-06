-- +goose Up
CREATE TABLE webhook_subscriptions (
	id UUID PRIMARY KEY DEFAULT GEN_RANDOM_UUID(),
	name TEXT NOT NULL CHECK (BTRIM(name) <> ''),
	team_slug slug REFERENCES teams (slug) ON DELETE CASCADE,
	source_team_slugs slug[],
	url TEXT NOT NULL,
	secret TEXT NOT NULL CHECK (CHAR_LENGTH(secret) >= 8),
	event_types TEXT[] NOT NULL,
	enabled BOOLEAN NOT NULL DEFAULT TRUE,
	consecutive_failures INT NOT NULL DEFAULT 0,
	disabled_at TIMESTAMPTZ,
	last_test_at TIMESTAMPTZ,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	CONSTRAINT webhook_subscriptions_team_name_key UNIQUE (team_slug, name),
	CONSTRAINT webhook_subscription_source_teams CHECK (
		team_slug IS NOT NULL
		OR COALESCE(CARDINALITY(source_team_slugs), 0) = 0
	)
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

-- Outbox table for durable webhook event processing. Rows are inserted by a trigger on
-- activity_log_entries. Subscription matching (event type wildcards, team scoping) happens
-- in the dispatcher, which fans each row out into per-subscription rows in
-- webhook_event_deliveries below.
CREATE TYPE webhook_outbox_status AS ENUM('pending', 'completed')
;

CREATE TABLE webhook_events (
	id UUID PRIMARY KEY DEFAULT GEN_RANDOM_UUID(),
	activity_log_entries_id UUID NOT NULL REFERENCES activity_log_entries (id) ON DELETE CASCADE,
	status webhook_outbox_status NOT NULL DEFAULT 'pending',
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)
;

CREATE INDEX idx_webhook_events_pending ON webhook_events (created_at ASC)
WHERE
	status = 'pending'
;

-- Per-(event, subscription) delivery queue; this is the unit of retry and backoff.
CREATE TYPE webhook_delivery_status AS ENUM('pending', 'processing', 'completed', 'failed')
;

CREATE TABLE webhook_event_deliveries (
	id UUID PRIMARY KEY DEFAULT GEN_RANDOM_UUID(),
	webhook_event_id UUID NOT NULL REFERENCES webhook_events (id) ON DELETE CASCADE,
	subscription_id UUID NOT NULL REFERENCES webhook_subscriptions (id) ON DELETE CASCADE,
	status webhook_delivery_status NOT NULL DEFAULT 'pending',
	retry_count INT NOT NULL DEFAULT 0,
	run_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	UNIQUE (webhook_event_id, subscription_id)
)
;

CREATE INDEX idx_webhook_event_deliveries_pending ON webhook_event_deliveries (run_at ASC)
WHERE
	status = 'pending'
;

CREATE INDEX idx_webhook_event_deliveries_event ON webhook_event_deliveries (webhook_event_id)
;

-- Audit log of every actual HTTP delivery attempt.
CREATE TABLE webhook_deliveries (
	id UUID PRIMARY KEY DEFAULT GEN_RANDOM_UUID(),
	subscription_id UUID NOT NULL REFERENCES webhook_subscriptions (id) ON DELETE CASCADE,
	webhook_event_delivery_id UUID REFERENCES webhook_event_deliveries (id) ON DELETE SET NULL,
	event_type TEXT NOT NULL,
	request_body BYTEA NOT NULL,
	request_headers JSONB NOT NULL DEFAULT '{}',
	response_status INT,
	response_body BYTEA,
	response_headers JSONB NOT NULL DEFAULT '{}',
	response_body_truncated BOOLEAN NOT NULL DEFAULT FALSE,
	error_message TEXT,
	duration_ms INT NOT NULL,
	success BOOLEAN NOT NULL,
	attempt_count INT NOT NULL DEFAULT 1 CHECK (attempt_count > 0),
	next_retry_at TIMESTAMPTZ,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)
;

CREATE INDEX idx_webhook_deliveries_subscription ON webhook_deliveries (subscription_id, created_at DESC)
;

CREATE INDEX idx_webhook_deliveries_event_delivery ON webhook_deliveries (webhook_event_delivery_id)
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
