-- +goose Up

CREATE TABLE dependencytrack_projects(
    id          uuid PRIMARY KEY,
    environment text NOT NULL,
    team_slug   slug NOT NULL REFERENCES teams(slug) ON DELETE CASCADE,
    app         text NOT NULL,
    created_at  timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE TABLE vulnerability_metrics(
    id                         uuid DEFAULT gen_random_uuid() PRIMARY KEY,
    date                       date             NOT NULL,
    dependencytrack_project_id uuid             NOT NULL REFERENCES dependencytrack_projects(id) ON DELETE CASCADE,
    critical                   integer          NOT NULL,
    high                       integer          NOT NULL,
    medium                     integer          NOT NULL,
    low                        integer          NOT NULL,
    unassigned                 integer          NOT NULL,
    risk_score                 double precision NOT NULL,
    CONSTRAINT vulnerability_metric UNIQUE (date, dependencytrack_project_id)
);