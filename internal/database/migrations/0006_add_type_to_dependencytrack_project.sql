-- +goose Up
DELETE FROM dependencytrack_projects T1
    USING   dependencytrack_projects T2
WHERE   T1.created_at < T2.created_at  -- delete the older versions
    AND T1.environment = T2.environment
    AND T1.team_slug = T2.team_slug
    AND T1.app = T2.app;

CREATE TYPE workload_type AS ENUM (
    'app',
    'naisjob'
);

ALTER TABLE dependencytrack_projects RENAME COLUMN app TO workload;

ALTER TABLE dependencytrack_projects ADD COLUMN workload_type workload_type;

UPDATE dependencytrack_projects SET workload_type = 'app';

ALTER TABLE dependencytrack_projects ALTER COLUMN workload_type SET NOT NULL;

CREATE UNIQUE INDEX dependencytrack_team_env_workload_type_idx ON dependencytrack_projects (team_slug, environment, workload, workload_type);

-- +goose Down
DROP INDEX dependencytrack_team_env_workload_type_idx;

ALTER TABLE dependencytrack_projects DROP COLUMN workload_type;

ALTER TABLE dependencytrack_projects RENAME COLUMN workload TO app;

DROP TYPE workload_type;
