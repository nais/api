-- +goose Up
ALTER TABLE reconciler_errors
ALTER COLUMN id
DROP DEFAULT,
ALTER COLUMN id TYPE TEXT
;

UPDATE reconciler_errors
SET
	id = GEN_RANDOM_UUID()
;

ALTER TABLE reconciler_errors
ALTER COLUMN id TYPE UUID USING (id::UUID),
ALTER COLUMN id
SET DEFAULT GEN_RANDOM_UUID()
;

DROP SEQUENCE reconciler_errors_id_seq
;
