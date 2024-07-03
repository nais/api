package graphv1

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/graphv1/donotuse"
	"github.com/nais/api/internal/graphv1/scalar"
	"github.com/nais/api/internal/slug"
)

func (r *queryResolver) Teams(ctx context.Context, first *int, after *scalar.Cursor, last *int, before *scalar.Cursor) (*donotuse.TeamConnection, error) {
	panic(fmt.Errorf("not implemented: Teams - teams"))
}

func (r *queryResolver) Team(ctx context.Context, slug slug.Slug) (*donotuse.Team, error) {
	panic(fmt.Errorf("not implemented: Team - team"))
}

func (r *queryResolver) TeamDeleteKey(ctx context.Context, key string) (*donotuse.TeamDeleteKey, error) {
	panic(fmt.Errorf("not implemented: TeamDeleteKey - teamDeleteKey"))
}
