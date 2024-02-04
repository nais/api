-- +goose Up

CREATE TABLE dependencytrack_projects(
    id          uuid PRIMARY KEY,
    environment text NOT NULL,
    team_slug   slug NOT NULL,
    app         text NOT NULL,
    projectId   uuid NOT NULL,
    CONSTRAINT dependencytrack_projects_key UNIQUE (environment, team_slug, app, projectId)
);

CREATE TABLE vulnerability_metrics(
    date                       date             NOT NULL,
    dependencytrack_project_id uuid             NOT NULL,
    critical                   integer          NOT NULL,
    high                       integer          NOT NULL,
    medium                     integer          NOT NULL,
    low                        integer          NOT NULL,
    unassigned                 integer          NOT NULL,
    risk_score                 double precision NOT NULL,
    PRIMARY KEY (date, dependencytrack_project_id),
    CONSTRAINT vulnerability_metrics UNIQUE (date, dependencytrack_project_id),
    CONSTRAINT fk_vulnerability_metrics_dependencytrack_project_id
        FOREIGN KEY (dependencytrack_project_id)
            REFERENCES dependencytrack_projects (id) ON DELETE CASCADE
);
