-- +goose Up

ALTER TABLE teams ADD deleted_at TIMESTAMPTZ DEFAULT NULL;

CREATE INDEX ON teams (deleted_at);

ALTER TABLE teams RENAME TO active_and_deleted_teams;

CREATE VIEW teams AS (
    SELECT *
    FROM active_and_deleted_teams
    WHERE deleted_at IS NULL
);

-- +goose Down

DROP VIEW teams;

ALTER TABLE active_and_deleted_teams RENAME TO teams;

DELETE FROM teams WHERE deleted_at IS NOT NULL;

ALTER TABLE teams DROP deleted_at;