package graph

import (
	"github.com/nais/api/internal/graph/gengql"
)

func (r *Resolver) Mutation() gengql.MutationResolver { return &mutationResolver{r} }

func (r *Resolver) Query() gengql.QueryResolver { return &queryResolver{r} }

type (
	mutationResolver struct{ *Resolver }
	queryResolver    struct{ *Resolver }
)
