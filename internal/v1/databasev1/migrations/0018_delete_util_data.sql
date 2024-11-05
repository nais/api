-- +goose Up
DROP MATERIALIZED VIEW IF EXISTS resource_team_range
;

DROP TABLE IF EXISTS resource_utilization_metrics
;
