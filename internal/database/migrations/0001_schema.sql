-- +goose Up
CREATE TABLE cost (
    id serial PRIMARY KEY,
    env text,
    team text,
    app text,
    cost_type text NOT NULL,
    date date NOT NULL,
    daily_cost real NOT NULL,
    CONSTRAINT daily_cost_key UNIQUE (env, team, app, cost_type, date)
);

CREATE INDEX cost_env_idx ON cost (env);
CREATE INDEX cost_team_idx ON cost (team);
CREATE INDEX cost_app_idx ON cost (app);
CREATE INDEX cost_date_idx ON cost (date);

-- +goose Down
DROP TABLE cost;