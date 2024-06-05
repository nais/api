-- +goose Up

CREATE TABLE audit_events (
    id uuid DEFAULT gen_random_uuid() PRIMARY KEY,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
	actor text NOT NULL,
	action text NOT NULL,
	resource_type text NOT NULL,
	resource_name text NOT NULL,
	team_slug slug
);

CREATE TABLE audit_events_data (
	event_id uuid NOT NULL,
	key text NOT NULL,
	value text NOT NULL,
	PRIMARY KEY (event_id, key),
	FOREIGN KEY (event_id) REFERENCES audit_events (id)
);
