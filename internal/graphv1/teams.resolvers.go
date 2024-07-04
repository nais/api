package graphv1

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/graphv1/gengqlv1"
	"github.com/nais/api/internal/graphv1/pagination"
	"github.com/nais/api/internal/graphv1/scalar"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/team"
)

func (r *queryResolver) Teams(ctx context.Context, first *int, after *scalar.Cursor, last *int, before *scalar.Cursor, orderBy *team.TeamOrder) (*pagination.Connection[*team.Team], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return team.List(ctx, page, orderBy)
}

func (r *queryResolver) Team(ctx context.Context, slug slug.Slug) (*team.Team, error) {
	return team.Get(ctx, slug)
}

func (r *teamResolver) DeletionInProgress(ctx context.Context, obj *team.Team) (bool, error) {
	return obj.DeleteKeyConfirmedAt != nil, nil
}

func (r *teamResolver) ViewerIsOwner(ctx context.Context, obj *team.Team) (bool, error) {
	panic(fmt.Errorf("not implemented: ViewerIsOwner - viewerIsOwner"))
}

func (r *teamResolver) ViewerIsMember(ctx context.Context, obj *team.Team) (bool, error) {
	panic(fmt.Errorf("not implemented: ViewerIsMember - viewerIsMember"))
}

func (r *Resolver) Team() gengqlv1.TeamResolver { return &teamResolver{r} }

type teamResolver struct{ *Resolver }
