-- +goose Up
DELETE FROM resource_utilization_metrics
WHERE usage = 0 OR request = 0;

ALTER TABLE resource_utilization_metrics
    ADD CONSTRAINT positive_usage CHECK (usage > 0),
    ADD CONSTRAINT positive_request CHECK (request > 0);

-- +goose Down
ALTER TABLE resource_utilization_metrics
    DROP CONSTRAINT positive_usage,
    DROP CONSTRAINT positive_request;