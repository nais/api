package graph

import (
	"context"

	"github.com/nais/api/internal/graph/model"
)

func (r *queryResolver) Search(ctx context.Context, query string, filter *model.SearchFilter, offset *int, limit *int) (*model.SearchList, error) {
	results := r.searcher.Search(ctx, query, filter)
	pagination := model.NewPagination(offset, limit)
	nodes, pi := model.PaginatedSlice(results, pagination)

	ret := &model.SearchList{
		PageInfo: pi,
	}

	for _, node := range nodes {
		ret.Nodes = append(ret.Nodes, node.Node)
	}

	return ret, nil
}
