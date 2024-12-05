package graph

import (
	"context"

	"github.com/davecgh/go-spew/spew"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/search"
)

func (r *queryResolver) Search(ctx context.Context, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, filter search.SearchFilter) (*pagination.Connection[search.SearchNode], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	ret, err := search.Search(ctx, page, filter)

	if filter.Query == "aura" {
		spew.Dump(ret)
	}

	return ret, err
}
