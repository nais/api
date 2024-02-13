-- +goose Up

CREATE TABLE dependencytrack_projects(
    id          uuid PRIMARY KEY,
    environment text NOT NULL,
    team_slug   slug NOT NULL REFERENCES teams(slug),
    app         text NOT NULL,
    created_at  timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT dependencytrack_projects_key UNIQUE (id)
);

CREATE TABLE vulnerability_metrics(
    id                         uuid DEFAULT gen_random_uuid() NOT NULL PRIMARY KEY,
    date                       date             NOT NULL,
    dependencytrack_project_id uuid             NOT NULL,
    critical                   integer          NOT NULL,
    high                       integer          NOT NULL,
    medium                     integer          NOT NULL,
    low                        integer          NOT NULL,
    unassigned                 integer          NOT NULL,
    risk_score                 double precision NOT NULL,
    CONSTRAINT vulnerability_metric UNIQUE (date, dependencytrack_project_id)
);

ALTER TABLE vulnerability_metrics
ADD FOREIGN KEY (dependencytrack_project_id) REFERENCES dependencytrack_projects(id) ON DELETE CASCADE;
