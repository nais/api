-- +goose Up
CREATE TABLE issues (
	id UUID DEFAULT gen_random_uuid () PRIMARY KEY,
	issue_type TEXT NOT NULL,
	resource_name TEXT NOT NULL,
	resource_type TEXT NOT NULL,
	team TEXT NOT NULL,
	env TEXT NOT NULL,
	severity TEXT NOT NULL DEFAULT 'todo',
	message TEXT NOT NULL DEFAULT '',
	issue_details JSONB,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)
;
