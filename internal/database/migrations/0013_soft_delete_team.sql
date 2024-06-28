-- +goose Up

ALTER TABLE teams
    ADD deleted_at TIMESTAMPTZ,
    ADD delete_key_confirmed_at TIMESTAMPTZ;

CREATE INDEX ON teams (deleted_at);
CREATE INDEX ON teams (delete_key_confirmed_at);

-- Set the value of the newly added column where applicable
UPDATE teams
SET delete_key_confirmed_at = team_delete_keys.confirmed_at
FROM team_delete_keys
WHERE
    team_delete_keys.team_slug = teams.slug
    AND team_delete_keys.confirmed_at IS NOT NULL;

-- +goose Down

DELETE FROM teams WHERE deleted_at IS NOT NULL;
ALTER TABLE teams DROP deleted_at;