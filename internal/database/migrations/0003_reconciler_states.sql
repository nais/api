-- +goose Up

DROP TABLE reconciler_resources;

CREATE TABLE reconciler_states (
    id UUID DEFAULT gen_random_uuid() NOT NULL PRIMARY KEY,
    reconciler_name text NOT NULL REFERENCES reconcilers(name) ON DELETE CASCADE,
    team_slug slug NOT NULL REFERENCES teams(slug) ON DELETE CASCADE,
    value bytea NOT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(reconciler_name, team_slug)
);

CREATE TRIGGER reconciler_states_set_updated BEFORE
UPDATE
    ON reconciler_states FOR EACH ROW EXECUTE PROCEDURE set_updated_at();