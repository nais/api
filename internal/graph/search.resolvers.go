package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.
// Code generated by github.com/99designs/gqlgen

import (
	"context"

	"github.com/nais/api/internal/graph/model"
)

// Search is the resolver for the search field.
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
