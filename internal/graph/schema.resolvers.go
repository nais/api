package graph

import (
	"context"

	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/model"
)

func (r *queryResolver) Node(ctx context.Context, id ident.Ident) (model.Node, error) {
	return ident.GetByIdent(ctx, id)
}

func (r *Resolver) Mutation() gengql.MutationResolver { return &mutationResolver{r} }

func (r *Resolver) Query() gengql.QueryResolver { return &queryResolver{r} }

func (r *Resolver) Subscription() gengql.SubscriptionResolver { return &subscriptionResolver{r} }

type mutationResolver struct{ *Resolver }
type queryResolver struct{ *Resolver }
type subscriptionResolver struct{ *Resolver }
