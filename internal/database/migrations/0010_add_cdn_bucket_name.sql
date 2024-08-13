-- +goose Up
ALTER TABLE teams
ADD COLUMN cdn_bucket TEXT
;

-- apparnetly we don't do down migrations
