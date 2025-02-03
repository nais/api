package graph

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/serviceaccount"
	"github.com/nais/api/internal/team"
)

func (r *queryResolver) ServiceAccounts(ctx context.Context, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor) (*pagination.Connection[*serviceaccount.ServiceAccount], error) {
	panic(fmt.Errorf("not implemented: ServiceAccounts - serviceAccounts"))
}

func (r *queryResolver) ServiceAccount(ctx context.Context, id ident.Ident) (*serviceaccount.ServiceAccount, error) {
	return serviceaccount.GetByIdent(ctx, id)
}

func (r *serviceAccountResolver) Team(ctx context.Context, obj *serviceaccount.ServiceAccount) (*team.Team, error) {
	if obj.TeamSlug == "" {
		return nil, nil
	}

	return team.Get(ctx, obj.TeamSlug)
}

func (r *Resolver) ServiceAccount() gengql.ServiceAccountResolver { return &serviceAccountResolver{r} }

type serviceAccountResolver struct{ *Resolver }
