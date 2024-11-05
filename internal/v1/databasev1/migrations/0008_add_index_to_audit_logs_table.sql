-- +goose Up
CREATE INDEX audit_logs_correlation_id_idx ON audit_logs (correlation_id)
;

-- +goose Down
DROP INDEX audit_logs_correlation_id_idx
;
