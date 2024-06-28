package graphv1

import (
	"github.com/nais/api/internal/graphv1/gengqlv1"
)

func (r *Resolver) Query() gengqlv1.QueryResolver { return &queryResolver{r} }

type queryResolver struct{ *Resolver }
