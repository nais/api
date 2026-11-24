-- +goose Up
-- Existing users are classified by the next successful user sync.
ALTER TABLE users
ADD COLUMN source TEXT CHECK (source IN ('TENANT', 'NAIS'))
;
