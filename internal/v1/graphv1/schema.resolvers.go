package graphv1

import (
	"context"

	"github.com/nais/api/internal/v1/graphv1/gengqlv1"
	"github.com/nais/api/internal/v1/graphv1/ident"
	"github.com/nais/api/internal/v1/graphv1/modelv1"
)

func (r *queryResolver) Node(ctx context.Context, id ident.Ident) (modelv1.Node, error) {
	return ident.GetByIdent(ctx, id)
}

func (r *Resolver) Mutation() gengqlv1.MutationResolver { return &mutationResolver{r} }

func (r *Resolver) Query() gengqlv1.QueryResolver { return &queryResolver{r} }

type (
	mutationResolver struct{ *Resolver }
	queryResolver    struct{ *Resolver }
)