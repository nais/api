-- +goose Up

ALTER TABLE teams ADD deleted_at TIMESTAMPTZ DEFAULT NULL;

CREATE INDEX ON teams (deleted_at);