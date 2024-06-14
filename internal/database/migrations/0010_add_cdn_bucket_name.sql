-- +goose Up
ALTER TABLE teams
ADD COLUMN cdn_bucket text;

-- apparnetly we don't do down migrations
