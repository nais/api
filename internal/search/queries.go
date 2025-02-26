package search

import (
	"context"

	"github.com/nais/api/internal/graph/pagination"
)

func Search(ctx context.Context, page *pagination.Pagination, filter SearchFilter) (*SearchNodeConnection, error) {
	return fromContext(ctx).searcher.Search(ctx, page, filter)
}
