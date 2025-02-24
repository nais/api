package search

import (
	"context"
)

var searchables []Searchers

type Searchers struct {
	Search func(ctx context.Context, q string) []*Result
	Type   SearchType
}

func Register(searchType SearchType, search func(ctx context.Context, q string) []*Result) {
	searchables = append(searchables, Searchers{Search: search, Type: searchType})
}
