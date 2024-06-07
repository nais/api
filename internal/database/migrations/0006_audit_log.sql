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
