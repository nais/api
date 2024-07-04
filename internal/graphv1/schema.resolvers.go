package graphv1

import (
	"context"

	"github.com/nais/api/internal/graphv1/gengqlv1"
	"github.com/nais/api/internal/graphv1/ident"
	"github.com/nais/api/internal/graphv1/modelv1"
)

func (r *queryResolver) Node(ctx context.Context, id ident.Ident) (modelv1.Node, error) {
	return ident.GetByIdent(ctx, id)
}

func (r *Resolver) Query() gengqlv1.QueryResolver { return &queryResolver{r} }

type queryResolver struct{ *Resolver }
