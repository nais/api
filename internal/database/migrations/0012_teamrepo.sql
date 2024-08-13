-- +goose Up
ALTER TABLE repository_authorizations
RENAME TO team_repositories
;

ALTER TABLE team_repositories
DROP COLUMN repository_authorization
;

ALTER TABLE "team_repositories"
ADD CONSTRAINT "team_repositories_team_slug_github_repository" PRIMARY KEY ("team_slug", "github_repository")
;

DROP TYPE repository_authorization_enum
;
