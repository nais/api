package sortfilter

import (
	"context"
	"fmt"
	"slices"

	"github.com/nais/api/internal/graph/model"
	"github.com/sourcegraph/conc/pool"
)

// SortFunc compares two values of type V and returns an integer indicating their order.
// If a < b, the function should return a negative value.
// If a == b, the function should return 0.
// If a > b, the function should return a positive value.
type SortFunc[V any] func(ctx context.Context, a, b V) int

// ConcurrentSortFunc should return an integer indicating the order of the given value.
// The results will later be sorted by the returned value.
type ConcurrentSortFunc[V any] func(ctx context.Context, a V) int

// Filter is a function that returns true if the given value should be included in the result.
type Filter[V any, FilterObj any] func(ctx context.Context, v V, filter FilterObj) bool

type funcs[V any] struct {
	concurrentSort ConcurrentSortFunc[V]
	sort           SortFunc[V]
}

type SortFilter[V any, SortField comparable, FilterObj comparable] struct {
	sorters           map[SortField]funcs[V]
	filters           []Filter[V, FilterObj]
	tieBreakSortField SortField
}

// New creates a new SortFilter with the given tieBreakSortField.
// The tieBreakSortField is used when two values are equal in the Sort function, and will use the direction supplied
// when calling Sort. The tieBreakSortField must not be registered as a ConcurrentSort.
func New[V any, SortField comparable, FilterObj comparable](tieBreakSortField SortField) *SortFilter[V, SortField, FilterObj] {
	return &SortFilter[V, SortField, FilterObj]{
		sorters:           make(map[SortField]funcs[V]),
		tieBreakSortField: tieBreakSortField,
	}
}

// SupportsSort returns true if the given field is registered using RegisterSort or RegisterConcurrentSort.
func (s *SortFilter[T, SortField, FilterObj]) SupportsSort(field SortField) bool {
	_, exists := s.sorters[field]
	return exists
}

func (s *SortFilter[T, SortField, FilterObj]) RegisterSort(field SortField, sort SortFunc[T]) {
	if _, ok := s.sorters[field]; ok {
		panic(fmt.Sprintf("sort field is already registered: %v", field))
	}

	s.sorters[field] = funcs[T]{
		sort: sort,
	}
}

func (s *SortFilter[T, SortField, FilterObj]) RegisterConcurrentSort(field SortField, sort ConcurrentSortFunc[T]) {
	if _, ok := s.sorters[field]; ok {
		panic(fmt.Sprintf("sort field is already registered: %v", field))
	} else if field == s.tieBreakSortField {
		panic(fmt.Sprintf("sort field is used for tie break and can not be concurrent: %v", field))
	}

	s.sorters[field] = funcs[T]{
		concurrentSort: sort,
	}
}

// RegisterFilter registers a filter function that will be applied to the items when calling Filter.
func (s *SortFilter[T, SortField, FilterObj]) RegisterFilter(filter Filter[T, FilterObj]) {
	s.filters = append(s.filters, filter)
}

// Filter filters all items based on the filters registered with RegisterFilter.
func (s *SortFilter[T, SortField, FilterObj]) Filter(ctx context.Context, items []T, filter FilterObj) []T {
	var nillish FilterObj
	if filter == nillish {
		return items
	}

	type ret struct {
		item    T
		include bool
	}

	wg := pool.NewWithResults[ret]().WithMaxGoroutines(50).WithContext(ctx)
	for _, item := range items {
		wg.Go(func(ctx context.Context) (ret, error) {
			for _, fn := range s.filters {
				if !fn(ctx, item, filter) {
					return ret{item: item, include: false}, nil
				}
			}
			return ret{item: item, include: true}, nil
		})
	}

	res, err := wg.Wait()
	if err != nil {
		// This should never happen, as filters doesn't return errors.
		panic(err)
	}

	filtered := make([]T, 0, len(res))
	for _, r := range res {
		if r.include {
			filtered = append(filtered, r.item)
		}
	}

	return filtered
}

// Sort will sort items based on a specific field and direction. The field used must be registered with RegisterSort or
// RegisterConcurrentSort.
func (s *SortFilter[T, SortField, FilterObj]) Sort(ctx context.Context, items []T, field SortField, direction model.OrderDirection) {
	sorter, ok := s.sorters[field]
	if !ok {
		panic(fmt.Sprintf("no sort registered for field: %v", field))
	}

	if len(items) == 0 {
		return
	}

	if sorter.concurrentSort != nil {
		s.sortConcurrent(ctx, items, sorter.concurrentSort, direction)
		return
	}

	s.sort(ctx, items, sorter.sort, direction)
}

func (s *SortFilter[T, SortField, FilterObj]) sortConcurrent(ctx context.Context, items []T, sort ConcurrentSortFunc[T], direction model.OrderDirection) {
	type sortable struct {
		item T
		key  int
	}

	wg := pool.NewWithResults[sortable]().WithMaxGoroutines(50).WithContext(ctx)
	for _, item := range items {
		wg.Go(func(ctx context.Context) (sortable, error) {
			return sortable{
				item: item,
				key:  sort(ctx, item),
			}, nil
		})
	}

	res, err := wg.Wait()
	if err != nil {
		// This should never happen, as sort doesn't return errors.
		panic(err)
	}

	slices.SortStableFunc(res, func(a, b sortable) int {
		if b.key == a.key {
			return s.tieBreak(ctx, a.item, b.item, direction)
		}

		if direction == model.OrderDirectionDesc {
			return b.key - a.key
		}
		return a.key - b.key
	})

	for i, r := range res {
		items[i] = r.item
	}
}

func (s *SortFilter[T, SortField, FilterObj]) sort(ctx context.Context, items []T, sort SortFunc[T], direction model.OrderDirection) {
	slices.SortStableFunc(items, func(a, b T) int {
		var ret int
		if direction == model.OrderDirectionDesc {
			ret = sort(ctx, b, a)
		} else {
			ret = sort(ctx, a, b)
		}

		if ret == 0 {
			return s.tieBreak(ctx, a, b, direction)
		}
		return ret
	})
}

func (s *SortFilter[T, SortField, FilterObj]) tieBreak(ctx context.Context, a, b T, direction model.OrderDirection) int {
	if direction == model.OrderDirectionDesc {
		return s.sorters[s.tieBreakSortField].sort(ctx, b, a)
	}

	return s.sorters[s.tieBreakSortField].sort(ctx, a, b)
}
