package database

import (
	"context"

	"github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/slug"
)

type TeamRepositoryRepo interface {
	AddTeamRepository(ctx context.Context, teamSlug slug.Slug, repoName string) error
	RemoveTeamRepository(ctx context.Context, teamSlug slug.Slug, repoName string) error
	ListTeamRepositories(ctx context.Context, teamSlug slug.Slug) ([]string, error)
	IsTeamRepository(ctx context.Context, teamSlug slug.Slug, repoName string) (bool, error)
}

func (d *database) AddTeamRepository(ctx context.Context, teamSlug slug.Slug, repoName string) error {
	return d.querier.AddTeamRepository(ctx, gensql.AddTeamRepositoryParams{
		TeamSlug:         teamSlug,
		GithubRepository: repoName,
	})
}

func (d *database) RemoveTeamRepository(ctx context.Context, teamSlug slug.Slug, repoName string) error {
	return d.querier.RemoveTeamRepository(ctx, gensql.RemoveTeamRepositoryParams{
		TeamSlug:         teamSlug,
		GithubRepository: repoName,
	})
}

func (d *database) ListTeamRepositories(ctx context.Context, teamSlug slug.Slug) ([]string, error) {
	return d.querier.GetTeamRepositories(ctx, teamSlug)
}

func (d *database) IsTeamRepository(ctx context.Context, teamSlug slug.Slug, repoName string) (bool, error) {
	return d.querier.IsTeamRepository(ctx, gensql.IsTeamRepositoryParams{
		TeamSlug:         teamSlug,
		GithubRepository: repoName,
	})
}
