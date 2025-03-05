-- +goose Up
CREATE TABLE k8s_events (
	uid UUID PRIMARY KEY,
	environment_name TEXT NOT NULL,
	involved_kind TEXT NOT NULL,
	involved_name TEXT NOT NULL,
	involved_namespace TEXT NOT NULL,
	data JSONB NOT NULL,
	reason TEXT NOT NULL,
	triggered_at TIMESTAMPTZ NOT NULL
)
;

CREATE INDEX ON k8s_events (
	environment_name,
	involved_kind,
	involved_name,
	involved_namespace
)
;
