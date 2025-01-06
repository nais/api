-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION unique_team_slug () RETURNS trigger AS $unique_team_slug$
    BEGIN
        IF (SELECT slug from team_slugs WHERE slug = NEW.slug) IS NOT NULL THEN
            RAISE 'Team slug is not available: %', NEW.slug
            USING ERRCODE = 'unique_violation';
        END IF;
        RETURN NEW;
    END;
$unique_team_slug$ LANGUAGE plpgsql
;

-- +goose StatementEnd
