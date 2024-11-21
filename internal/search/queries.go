package search

import (
	"context"
	"slices"
	"sort"
	"strings"

	"github.com/nais/api/internal/graph/pagination"
	"github.com/sourcegraph/conc/pool"
)

func Search(ctx context.Context, page *pagination.Pagination, filter SearchFilter) (*SearchNodeConnection, error) {
	q := strings.TrimSpace(filter.Query)
	if q == "" {
		return pagination.EmptyConnection[SearchNode](), nil
	}

	wg := pool.NewWithResults[[]*Result]().WithMaxGoroutines(5)
	for _, searchable := range searchables {
		if filter.Type != nil && searchable.Type != *filter.Type {
			continue
		}

		wg.Go(func() []*Result {
			return searchable.Search(ctx, q)
		})
	}

	wgRet := wg.Wait()
	ret := make([]*Result, 0)
	for _, r := range wgRet {
		ret = append(ret, r...)
	}

	ret = slices.DeleteFunc(ret, func(e *Result) bool {
		return !Include(e.Rank)
	})

	sort.SliceStable(ret, func(i, j int) bool {
		return ret[i].Rank < ret[j].Rank
	})

	return pagination.NewConvertConnection(pagination.Slice(ret, page), page, len(ret), func(from *Result) SearchNode {
		return from.Node
	}), nil
}
