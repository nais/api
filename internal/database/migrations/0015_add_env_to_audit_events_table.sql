-- +goose Up
ALTER TABLE audit_events
ADD COLUMN environment TEXT
;

-- +goose Down
ALTER TABLE audit_events
DROP COLUMN environment
;
