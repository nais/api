-- +goose Up
ALTER TABLE team_environments
ADD COLUMN cdn_bucket text;

-- apparnetly we don't do down migrations
