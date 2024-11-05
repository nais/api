-- +goose Up
ALTER TABLE audit_logs
DROP COLUMN component_name
;

-- +goose Down
ALTER TABLE audit_logs
ADD COLUMN component_name TEXT NOT NULL DEFAULT 'unknown'
;
