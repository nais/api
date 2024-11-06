package graph

import (
	"context"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/github/repository"
	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/role"
	"github.com/nais/api/internal/team"
)

func (r *mutationResolver) AddRepositoryToTeam(ctx context.Context, input repository.AddRepositoryToTeamInput) (*repository.AddRepositoryToTeamPayload, error) {
	if err := authz.RequireTeamAuthorizationCtx(ctx, role.AuthorizationRepositoriesCreate, input.TeamSlug); err != nil {
		return nil, err
	}
	repo, err := repository.Create(ctx, input)
	if err != nil {
		return nil, err
	}

	return &repository.AddRepositoryToTeamPayload{
		Repository: repo,
	}, nil
}

func (r *mutationResolver) RemoveRepositoryFromTeam(ctx context.Context, input repository.RemoveRepositoryFromTeamInput) (*repository.RemoveRepositoryFromTeamPayload, error) {
	if err := authz.RequireTeamAuthorizationCtx(ctx, role.AuthorizationRepositoriesDelete, input.TeamSlug); err != nil {
		return nil, err
	}
	err := repository.Remove(ctx, input)
	return &repository.RemoveRepositoryFromTeamPayload{
		Success: err == nil,
	}, err
}

func (r *repositoryResolver) Team(ctx context.Context, obj *repository.Repository) (*team.Team, error) {
	return team.Get(ctx, obj.TeamSlug)
}

func (r *teamResolver) Repositories(ctx context.Context, obj *team.Team, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *repository.RepositoryOrder, filter *repository.TeamRepositoryFilter) (*pagination.Connection[*repository.Repository], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}
	return repository.ListForTeam(ctx, obj.Slug, page, orderBy, filter)
}

func (r *Resolver) Repository() gengql.RepositoryResolver { return &repositoryResolver{r} }

type repositoryResolver struct{ *Resolver }
