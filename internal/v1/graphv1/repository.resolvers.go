package graphv1

import (
	"context"

	"github.com/nais/api/internal/v1/github/repository"
	"github.com/nais/api/internal/v1/graphv1/gengqlv1"
	"github.com/nais/api/internal/v1/graphv1/pagination"
	"github.com/nais/api/internal/v1/team"
)

func (r *mutationResolver) AddRepositoryToTeam(ctx context.Context, input repository.AddRepositoryToTeamInput) (*repository.AddRepositoryToTeamPayload, error) {
	repo, err := repository.Create(ctx, input)
	if err != nil {
		return nil, err
	}

	return &repository.AddRepositoryToTeamPayload{
		Repository: repo,
	}, nil
}

func (r *mutationResolver) RemoveRepositoryFromTeam(ctx context.Context, input repository.RemoveRepositoryFromTeamInput) (*repository.RemoveRepositoryFromTeamPayload, error) {
	err := repository.Remove(ctx, input)
	return &repository.RemoveRepositoryFromTeamPayload{
		Success: err == nil,
	}, err
}

func (r *repositoryResolver) Team(ctx context.Context, obj *repository.Repository) (*team.Team, error) {
	return team.Get(ctx, obj.TeamSlug)
}

func (r *teamResolver) Repositories(ctx context.Context, obj *team.Team, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, filter *repository.TeamRepositoryFilter) (*pagination.Connection[*repository.Repository], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}
	return repository.ListForTeam(ctx, obj.Slug, page, filter)
}

func (r *Resolver) Repository() gengqlv1.RepositoryResolver { return &repositoryResolver{r} }

type repositoryResolver struct{ *Resolver }
