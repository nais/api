-- +goose Up
ALTER TABLE cost
RENAME COLUMN app TO app_label
;
