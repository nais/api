package database

import (
	"context"

	sqlc "github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/slug"
)

type RepositoryAuthorizationRepo interface {
	CreateRepositoryAuthorization(ctx context.Context, teamSlug slug.Slug, repoName string, authorization sqlc.RepositoryAuthorizationEnum) error
	GetRepositoryAuthorizations(ctx context.Context, teamSlug slug.Slug, repo string) ([]sqlc.RepositoryAuthorizationEnum, error)
	RemoveRepositoryAuthorization(ctx context.Context, teamSlug slug.Slug, repoName string, authorization sqlc.RepositoryAuthorizationEnum) error
}

func (d *database) CreateRepositoryAuthorization(ctx context.Context, teamSlug slug.Slug, repoName string, authorization sqlc.RepositoryAuthorizationEnum) error {
	return d.querier.CreateRepositoryAuthorization(ctx, teamSlug, repoName, authorization)
}

func (d *database) RemoveRepositoryAuthorization(ctx context.Context, teamSlug slug.Slug, repoName string, authorization sqlc.RepositoryAuthorizationEnum) error {
	return d.querier.RemoveRepositoryAuthorization(ctx, teamSlug, repoName, authorization)
}

func (d *database) GetRepositoryAuthorizations(ctx context.Context, teamSlug slug.Slug, repo string) ([]sqlc.RepositoryAuthorizationEnum, error) {
	return d.querier.GetRepositoryAuthorizations(ctx, teamSlug, repo)
}
