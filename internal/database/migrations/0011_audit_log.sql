-- +goose Up
CREATE TABLE audit_events (
	id UUID DEFAULT gen_random_uuid () PRIMARY KEY,
	created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
	actor TEXT NOT NULL,
	action TEXT NOT NULL,
	resource_type TEXT NOT NULL,
	resource_name TEXT NOT NULL,
	team_slug slug,
	data bytea
)
;

CREATE INDEX audit_events_team_slug_idx ON audit_events (team_slug)
;

CREATE INDEX audit_events_resource_type_idx ON audit_events (resource_type)
;

CREATE INDEX audit_events_created_at_idx ON audit_events (created_at)
;

-- +goose Down
DROP TABLE audit_events
;
