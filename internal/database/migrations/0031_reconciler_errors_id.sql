-- +goose Up
ALTER TABLE reconciler_errors
ALTER COLUMN id
DROP DEFAULT,
ALTER COLUMN id TYPE TEXT
;

UPDATE reconciler_errors
SET
	id = gen_random_uuid ()
;

ALTER TABLE reconciler_errors
ALTER COLUMN id TYPE UUID USING (id::UUID),
ALTER COLUMN id
SET DEFAULT gen_random_uuid ()
;

DROP SEQUENCE reconciler_errors_id_seq
;
