-- +goose Up

CREATE TABLE team_slugs (
    slug slug PRIMARY KEY
);
INSERT INTO team_slugs (slug) SELECT slug FROM teams;

ALTER TABLE teams
    ADD delete_key_confirmed_at TIMESTAMPTZ;

CREATE INDEX ON teams (delete_key_confirmed_at);

UPDATE teams
SET delete_key_confirmed_at = team_delete_keys.confirmed_at
FROM team_delete_keys
WHERE
    team_delete_keys.team_slug = teams.slug
    AND team_delete_keys.confirmed_at IS NOT NULL;

-- +goose StatementBegin
CREATE FUNCTION unique_team_slug() RETURNS trigger AS $unique_team_slug$
    BEGIN
        IF (SELECT slug from team_slugs WHERE slug = NEW.slug) IS NOT NULL THEN
            RAISE EXCEPTION 'team slug is not availble: %', NEW.slug;
        END IF;
        RETURN NEW;
    END;
$unique_team_slug$ LANGUAGE plpgsql;

CREATE FUNCTION register_team_slug() RETURNS trigger AS $register_team_slug$
    BEGIN
        INSERT INTO team_slugs (slug) VALUES (NEW.slug);
        RETURN NEW;
    END;
$register_team_slug$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER unique_team_slug BEFORE INSERT ON teams
    FOR EACH ROW EXECUTE PROCEDURE unique_team_slug();
CREATE TRIGGER register_slug AFTER INSERT ON teams
    FOR EACH ROW EXECUTE PROCEDURE register_team_slug();
