-- +goose Up
CREATE TABLE usersync_runs (
	id UUID PRIMARY KEY,
	started_at TIMESTAMP WITH TIME ZONE NOT NULL,
	finished_at TIMESTAMP WITH TIME ZONE NOT NULL,
	error TEXT
)
;

CREATE INDEX ON usersync_runs USING btree (started_at DESC)
;

-- +goose Down
DROP TABLE usersync_runs
;
