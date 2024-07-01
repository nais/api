-- +goose Up

ALTER TABLE teams
    ADD deleted_at TIMESTAMPTZ,
    ADD delete_key_confirmed_at TIMESTAMPTZ;

CREATE INDEX ON teams (deleted_at);
CREATE INDEX ON teams (delete_key_confirmed_at);

UPDATE teams
SET delete_key_confirmed_at = team_delete_keys.confirmed_at
FROM team_delete_keys
WHERE
    team_delete_keys.team_slug = teams.slug
    AND team_delete_keys.confirmed_at IS NOT NULL;

ALTER TABLE teams RENAME TO all_teams_including_deleted;
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
        cdn_bucket,
        delete_key_confirmed_at
    FROM
        all_teams_including_deleted
    WHERE
        deleted_at IS NULL
);

-- +goose Down

DROP VIEW teams;

ALTER TABLE all_teams_including_deleted RENAME TO teams;

DELETE FROM teams WHERE deleted_at IS NOT NULL;

ALTER TABLE teams
    DROP deleted_at,
    DROP delete_key_confirmed_at;
