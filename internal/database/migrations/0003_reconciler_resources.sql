-- +goose Up
CREATE TABLE reconciler_resources (
  id UUID DEFAULT gen_random_uuid() NOT NULL PRIMARY KEY,
  reconciler_name text NOT NULL REFERENCES reconcilers(name),
  team_slug slug NOT NULL REFERENCES teams(slug),
  name TEXT NOT NULL,
  value TEXT NOT NULL,
  metadata JSONB NOT NULL,
  created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX ON reconciler_resources (reconciler_name, name, team_slug);

CREATE TRIGGER reconciler_resources_set_updated
    BEFORE UPDATE
    ON reconciler_resources
    FOR EACH ROW
    EXECUTE PROCEDURE set_updated_at();
