package search

import (
	"context"
	"sort"

	"github.com/lithammer/fuzzysearch/fuzzy"
	"github.com/nais/api/internal/graph/model"
)

type Searchable interface {
	SupportsSearchFilter(filter *model.SearchFilter) bool
	Search(ctx context.Context, q string, filter *model.SearchFilter) []*Result
}

type Result struct {
	Node model.SearchNode
	Rank int
}

type Searcher struct {
	searchables []Searchable
}

func New(s ...Searchable) *Searcher {
	return &Searcher{searchables: s}
}

func (s *Searcher) Search(ctx context.Context, q string, filter *model.SearchFilter) []*Result {
	ret := make([]*Result, 0)
	for _, searchable := range s.searchables {
		if !searchable.SupportsSearchFilter(filter) {
			continue
		}

		results := searchable.Search(ctx, q, filter)
		ret = append(ret, results...)
	}

	sort.SliceStable(ret, func(i, j int) bool {
		return ret[i].Rank < ret[j].Rank
	})

	return ret
}

// Match returns the rank of a match between q and val. 0 means best match. -1 means no match.
func Match(q, val string) int {
	return fuzzy.RankMatchFold(q, val)
}
