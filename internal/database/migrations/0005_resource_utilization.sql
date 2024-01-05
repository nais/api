-- +goose Up
CREATE TYPE resource_type AS ENUM (
    'cpu',
    'memory'
);

CREATE TABLE resource_utilization_metrics (
    id serial PRIMARY KEY,
    timestamp timestamp with time zone NOT NULL,
    env text NOT NULL,
    team text NOT NULL,
    app text NOT NULL,
    resource_type resource_type NOT NULL,
    usage double precision NOT NULL,
    request double precision NOT NULL,
    CONSTRAINT resource_utilization_metric UNIQUE (timestamp, env, team, app, resource_type)
);

CREATE INDEX ON resource_utilization_metrics (app);
CREATE INDEX ON resource_utilization_metrics (env);
CREATE INDEX ON resource_utilization_metrics (resource_type);
CREATE INDEX ON resource_utilization_metrics (team);
CREATE INDEX ON resource_utilization_metrics (timestamp);

-- +goose Down
DROP TABLE resource_utilization_metrics;
DROP TYPE resource_type;