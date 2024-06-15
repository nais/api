-- +goose Up

ALTER TABLE teams ADD deleted_at TIMESTAMPTZ;

CREATE INDEX ON teams (deleted_at);

ALTER TABLE teams RENAME TO active_or_deleted_teams;

-- Create a view that only shows active teams. The view will include teams that are "in deletion" (as in having a
-- confirmed delete key), but not teams that has been marked as deleted through the `deleted_at` column.
CREATE VIEW teams AS (
    SELECT
        slug,
        purpose,
        last_successful_sync,
        slack_channel,
        google_group_email,
        azure_group_id,
        github_team_slug,
        gar_repository,
        cdn_bucket
    FROM active_or_deleted_teams
    WHERE deleted_at IS NULL
);

-- +goose Down

DROP VIEW teams;

ALTER TABLE active_or_deleted_teams RENAME TO teams;

DELETE FROM teams WHERE deleted_at IS NOT NULL;

ALTER TABLE teams DROP deleted_at;