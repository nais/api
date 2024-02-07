-- +goose Up

CREATE TABLE dependencytrack_projects(
    id          uuid DEFAULT gen_random_uuid() NOT NULL,
    environment text NOT NULL,
    team_slug   slug NOT NULL,
    app         text NOT NULL,
    projectId   uuid NOT NULL,
    created_at  timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    PRIMARY KEY (id),
    UNIQUE(projectId),
    CONSTRAINT dependencytrack_projects_key UNIQUE (environment, team_slug, app, projectId)
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
ADD FOREIGN KEY (dependencytrack_project_id) REFERENCES dependencytrack_projects(projectId) ON DELETE CASCADE;

CREATE INDEX ON dependencytrack_projects (projectId);
CREATE INDEX ON dependencytrack_projects (environment);
CREATE INDEX ON dependencytrack_projects (team_slug);
CREATE INDEX ON dependencytrack_projects (app);
