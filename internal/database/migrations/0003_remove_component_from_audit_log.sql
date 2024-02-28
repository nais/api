-- +goose Up

ALTER TABLE audit_logs DROP COLUMN component_name;

-- +goose Down

ALTER TABLE audit_logs ADD COLUMN component_name text NOT NULL DEFAULT 'unknown';
