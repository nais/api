-- +goose Up
CREATE TABLE departments (
	id UUID DEFAULT gen_random_uuid () PRIMARY KEY,
	created_at TIMESTAMP WITH TIME ZONE DEFAULT CLOCK_TIMESTAMP() NOT NULL,
	slug slug NOT NULL,
	purpose TEXT NOT NULL,
	slack_channel TEXT NOT NULL
)
;
