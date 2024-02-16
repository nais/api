package database

import (
	"context"

	"github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/slug"
)

type RepositoryAuthorizationRepo interface {
	CreateRepositoryAuthorization(ctx context.Context, teamSlug slug.Slug, repoName string, authorization gensql.RepositoryAuthorizationEnum) error
	GetRepositoryAuthorizations(ctx context.Context, teamSlug slug.Slug, repoName string) ([]gensql.RepositoryAuthorizationEnum, error)
	RemoveRepositoryAuthorization(ctx context.Context, teamSlug slug.Slug, repoName string, authorization gensql.RepositoryAuthorizationEnum) error
	ListRepositoriesByAuthorization(ctx context.Context, teamSlug slug.Slug, authorization gensql.RepositoryAuthorizationEnum) ([]string, error)
}

func (d *database) CreateRepositoryAuthorization(ctx context.Context, teamSlug slug.Slug, repoName string, authorization gensql.RepositoryAuthorizationEnum) error {
	return d.querier.CreateRepositoryAuthorization(ctx, gensql.CreateRepositoryAuthorizationParams{
		TeamSlug:                teamSlug,
		GithubRepository:        repoName,
		RepositoryAuthorization: authorization,
	})
}

func (d *database) RemoveRepositoryAuthorization(ctx context.Context, teamSlug slug.Slug, repoName string, authorization gensql.RepositoryAuthorizationEnum) error {
	return d.querier.RemoveRepositoryAuthorization(ctx, gensql.RemoveRepositoryAuthorizationParams{
		TeamSlug:                teamSlug,
		GithubRepository:        repoName,
		RepositoryAuthorization: authorization,
	})
}

func (d *database) GetRepositoryAuthorizations(ctx context.Context, teamSlug slug.Slug, repoName string) ([]gensql.RepositoryAuthorizationEnum, error) {
	return d.querier.GetRepositoryAuthorizations(ctx, gensql.GetRepositoryAuthorizationsParams{
		TeamSlug:         teamSlug,
		GithubRepository: repoName,
	})
}

func (d *database) ListRepositoriesByAuthorization(ctx context.Context, teamSlug slug.Slug, authorization gensql.RepositoryAuthorizationEnum) ([]string, error) {
	return d.querier.ListRepositoriesByAuthorization(ctx, gensql.ListRepositoriesByAuthorizationParams{
		TeamSlug:                teamSlug,
		RepositoryAuthorization: authorization,
	})
}
