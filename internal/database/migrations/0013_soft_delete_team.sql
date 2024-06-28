-- +goose Up

ALTER TABLE teams
    ADD deleted_at TIMESTAMPTZ,
    ADD delete_key_confirmed_at TIMESTAMPTZ;

CREATE INDEX ON teams (deleted_at);

-- +goose Down

DELETE FROM teams WHERE deleted_at IS NOT NULL;
ALTER TABLE teams DROP deleted_at;