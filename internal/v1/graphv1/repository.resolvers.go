package graphv1

import (
	"context"

	"github.com/nais/api/internal/v1/github/repository"
	"github.com/nais/api/internal/v1/graphv1/gengqlv1"
	"github.com/nais/api/internal/v1/graphv1/pagination"
	"github.com/nais/api/internal/v1/team"
)

func (r *repositoryResolver) Team(ctx context.Context, obj *repository.Repository) (*team.Team, error) {
	return team.Get(ctx, obj.TeamSlug)
}

func (r *teamResolver) Repositories(ctx context.Context, obj *team.Team, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor) (*pagination.Connection[*repository.Repository], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}
	return repository.ListForTeam(ctx, obj.Slug, page)
}

func (r *Resolver) Repository() gengqlv1.RepositoryResolver { return &repositoryResolver{r} }

type repositoryResolver struct{ *Resolver }
