-- +goose Up
ALTER TABLE cost
RENAME COLUMN cost_type TO service
;
