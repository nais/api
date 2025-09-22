-- +goose Up
CREATE TYPE severity_level AS ENUM('CRITICAL', 'WARNING', 'TODO')
;

ALTER TABLE issues
ALTER COLUMN severity
DROP DEFAULT,
ALTER COLUMN severity TYPE severity_level USING severity::severity_level,
ALTER COLUMN severity
SET NOT NULL
;
