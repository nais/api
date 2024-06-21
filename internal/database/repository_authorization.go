package database

import (
	"context"

	"github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/slug"
)

type RepositoryAuthorizationRepo interface {
	CreateRepositoryAuthorization(ctx context.Context, teamSlug slug.Slug, repoName string) error
	RemoveRepositoryAuthorization(ctx context.Context, teamSlug slug.Slug, repoName string) error
	ListAuthorizedRepositories(ctx context.Context, teamSlug slug.Slug) ([]string, error)
	IsRepositoryAuthorized(ctx context.Context, teamSlug slug.Slug, repoName string) (bool, error)
}

func (d *database) CreateRepositoryAuthorization(ctx context.Context, teamSlug slug.Slug, repoName string) error {
	return d.querier.CreateRepositoryAuthorization(ctx, gensql.CreateRepositoryAuthorizationParams{
		TeamSlug:         teamSlug,
		GithubRepository: repoName,
	})
}

func (d *database) RemoveRepositoryAuthorization(ctx context.Context, teamSlug slug.Slug, repoName string) error {
	return d.querier.RemoveRepositoryAuthorization(ctx, gensql.RemoveRepositoryAuthorizationParams{
		TeamSlug:         teamSlug,
		GithubRepository: repoName,
	})
}

func (d *database) ListAuthorizedRepositories(ctx context.Context, teamSlug slug.Slug) ([]string, error) {
	return d.querier.GetAuthorizedRepositories(ctx, teamSlug)
}

func (d *database) IsRepositoryAuthorized(ctx context.Context, teamSlug slug.Slug, repoName string) (bool, error) {
	return d.querier.IsRepositoryAuthorized(ctx, gensql.IsRepositoryAuthorizedParams{
		TeamSlug:         teamSlug,
		GithubRepository: repoName,
	})
}
