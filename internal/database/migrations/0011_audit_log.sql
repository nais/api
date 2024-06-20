-- +goose Up

CREATE TABLE audit_events (
    id uuid DEFAULT gen_random_uuid() PRIMARY KEY,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    actor text NOT NULL,
    action text NOT NULL,
    resource_type text NOT NULL,
    resource_name text NOT NULL,
    team_slug slug,
    data bytea
);

CREATE INDEX audit_events_team_slug_idx ON audit_events (team_slug);
CREATE INDEX audit_events_resource_type_idx ON audit_events (resource_type);
CREATE INDEX audit_events_created_at_idx ON audit_events (created_at);

-- +goose Down

DROP TABLE audit_events;
