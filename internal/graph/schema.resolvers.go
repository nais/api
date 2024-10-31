package graph

import (
	"github.com/nais/api/internal/graph/gengql"
)

func (r *Resolver) Query() gengql.QueryResolver { return &queryResolver{r} }

type queryResolver struct{ *Resolver }
