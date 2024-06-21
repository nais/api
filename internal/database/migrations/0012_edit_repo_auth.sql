-- +goose Up

ALTER TABLE repository_authorizations DROP COLUMN repository_authorization;

ALTER TABLE "repository_authorizations"
ADD CONSTRAINT "repository_authorizations_team_slug_github_repository" PRIMARY KEY ("team_slug", "github_repository");

DROP TYPE repository_authorization_enum;
