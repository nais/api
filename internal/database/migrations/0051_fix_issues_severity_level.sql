-- +goose Up
CREATE TYPE severity_level_new AS ENUM('TODO', 'WARNING', 'CRITICAL')
;

ALTER TABLE issues
ALTER COLUMN severity TYPE severity_level_new USING severity::TEXT::severity_level_new
;

DROP TYPE severity_level
;

ALTER TYPE severity_level_new
RENAME TO severity_level
;
