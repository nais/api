-- +goose Up
ALTER TABLE webhook_subscriptions
ADD COLUMN source_team_slugs slug[]
;

UPDATE webhook_subscriptions
SET
	source_team_slugs = ARRAY[team_slug]
WHERE
	team_slug IS NOT NULL
;

ALTER TABLE webhook_subscriptions
ADD CONSTRAINT webhook_subscription_source_teams CHECK (
	team_slug IS NOT NULL
	OR COALESCE(CARDINALITY(source_team_slugs), 0) = 0
)
;

-- +goose Down
ALTER TABLE webhook_subscriptions
DROP CONSTRAINT webhook_subscription_source_teams,
DROP COLUMN source_team_slugs
;
