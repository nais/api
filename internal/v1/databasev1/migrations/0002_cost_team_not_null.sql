-- +goose Up
DELETE FROM cost
WHERE
	team_slug IS NULL
;

ALTER TABLE cost
ALTER COLUMN team_slug
SET NOT NULL
;
