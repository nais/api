package graphv1

import (
	"context"

	"github.com/nais/api/internal/v1/graphv1/pagination"
	"github.com/nais/api/internal/v1/searchv1"
)

func (r *queryResolver) Search(ctx context.Context, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, filter searchv1.SearchFilter) (*pagination.Connection[searchv1.SearchNode], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return searchv1.Search(ctx, page, filter)
}
