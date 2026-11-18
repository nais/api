-- +goose NO TRANSACTION
-- +goose Up
CREATE INDEX CONCURRENTLY IF NOT EXISTS k8s_events_triggered_at_idx ON k8s_events (triggered_at)
;
