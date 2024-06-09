-- +goose Up

CREATE TABLE usersync_runs (
    id uuid PRIMARY KEY,
    started_at timestamp with time zone NOT NULL,
    finished_at timestamp with time zone NOT NULL,
    error text
);

CREATE INDEX ON usersync_runs USING btree (started_at DESC);

-- +goose Down

DROP TABLE usersync_runs;