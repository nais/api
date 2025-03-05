package sortfilter

import (
	"context"
	"fmt"
	"slices"

	"github.com/nais/api/internal/graph/model"
	"github.com/sirupsen/logrus"
	"github.com/sourcegraph/conc/pool"
)

// SortFunc compares two values of type V and returns an integer indicating their order.
// If a < b, the function should return a negative value.
// If a == b, the function should return 0.
// If a > b, the function should return a positive value.
type SortFunc[T any] func(ctx context.Context, a, b T) int

// ConcurrentSortFunc should return an integer indicating the order of the given value.
// The results will later be sorted by the returned value.
type ConcurrentSortFunc[T any] func(ctx context.Context, a T) int

// Filter is a function that returns true if the given value should be included in the result.
type Filter[T any, FilterObj any] func(ctx context.Context, v T, filter FilterObj) bool

// TieBreaker is a combination of a SortField and a direction that might be able to resolve equal fields during sorting.
// If the direction is not supplied, the direction used for the original sort will be used. The referenced field must be
// registered with RegisterSort (concurrent tie-break sorters are not supported).
type TieBreaker[SortField comparable] struct {
	Field     SortField
	Direction *model.OrderDirection
}

type funcs[T any, SortField comparable] struct {
	concurrentSort ConcurrentSortFunc[T]
	sort           SortFunc[T]
	tieBreakers    []TieBreaker[SortField]
}

type SortFilter[T any, SortField comparable, FilterObj comparable] struct {
	sorters map[SortField]funcs[T, SortField]
	filters []Filter[T, FilterObj]
}

// New creates a new SortFilter
func New[T any, SortField comparable, FilterObj comparable]() *SortFilter[T, SortField, FilterObj] {
	return &SortFilter[T, SortField, FilterObj]{
		sorters: make(map[SortField]funcs[T, SortField]),
	}
}

// SupportsSort returns true if the given field is registered using RegisterSort or RegisterConcurrentSort.
func (s *SortFilter[T, SortField, FilterObj]) SupportsSort(field SortField) bool {
	_, exists := s.sorters[field]
	return exists
}

// RegisterSort will add support for sorting on a specific field. Optional tie-breakers can be supplied to resolve equal
// values, and will be executed in the given order.
func (s *SortFilter[T, SortField, FilterObj]) RegisterSort(field SortField, sort SortFunc[T], tieBreakers ...TieBreaker[SortField]) {
	if _, ok := s.sorters[field]; ok {
		panic(fmt.Sprintf("sort field is already registered: %v", field))
	}

	s.sorters[field] = funcs[T, SortField]{
		sort:        sort,
		tieBreakers: tieBreakers,
	}
}

// RegisterConcurrentSort will add support for doing concurrent sorting on a specific field. Optional tie-breakers can
// be supplied to resolve equal values, and will be executed in the given order.
func (s *SortFilter[T, SortField, FilterObj]) RegisterConcurrentSort(field SortField, sort ConcurrentSortFunc[T], tieBreakers ...TieBreaker[SortField]) {
	if _, ok := s.sorters[field]; ok {
		panic(fmt.Sprintf("sort field is already registered: %v", field))
	}

	s.sorters[field] = funcs[T, SortField]{
		concurrentSort: sort,
		tieBreakers:    tieBreakers,
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
		s.sortConcurrent(ctx, items, sorter.concurrentSort, field, direction, sorter.tieBreakers...)
		return
	}

	s.sort(ctx, items, sorter.sort, field, direction, sorter.tieBreakers...)
}

func (s *SortFilter[T, SortField, FilterObj]) sortConcurrent(ctx context.Context, items []T, sort ConcurrentSortFunc[T], field SortField, direction model.OrderDirection, tieBreakers ...TieBreaker[SortField]) {
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
			return s.tieBreak(ctx, a.item, b.item, field, direction, tieBreakers...)
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

func (s *SortFilter[T, SortField, FilterObj]) sort(ctx context.Context, items []T, sort SortFunc[T], field SortField, direction model.OrderDirection, tieBreakers ...TieBreaker[SortField]) {
	slices.SortStableFunc(items, func(a, b T) int {
		var ret int
		if direction == model.OrderDirectionDesc {
			ret = sort(ctx, b, a)
		} else {
			ret = sort(ctx, a, b)
		}

		if ret == 0 {
			return s.tieBreak(ctx, a, b, field, direction, tieBreakers...)
		}
		return ret
	})
}

// tieBreak will resolve equal fields after the initial sort by using the supplied tie-breakers. The function will
// return as soon as a tie-breaker returns a non-zero value.
func (s *SortFilter[T, SortField, FilterObj]) tieBreak(ctx context.Context, a, b T, field SortField, direction model.OrderDirection, tieBreakers ...TieBreaker[SortField]) int {
	for _, tb := range tieBreakers {
		dir := direction
		if tb.Direction != nil {
			dir = *tb.Direction
		}

		sorter, ok := s.sorters[tb.Field]
		if !ok {
			logrus.WithFields(logrus.Fields{
				"field_type":  fmt.Sprintf("%T", field),
				"tie_breaker": tb.Field,
			}).Errorf("no sort registered for tie-breaker")
			continue
		} else if sorter.sort == nil {
			logrus.WithFields(logrus.Fields{
				"field_type":  fmt.Sprintf("%T", field),
				"tie_breaker": tb.Field,
			}).Errorf("tie-breaker can not be a concurrent sort")
			continue
		}

		var v int
		if dir == model.OrderDirectionDesc {
			v = sorter.sort(ctx, b, a)
		} else {
			v = sorter.sort(ctx, a, b)
		}

		if v != 0 {
			return v
		}
	}

	logrus.
		WithFields(logrus.Fields{
			"field_type":   fmt.Sprintf("%T", field),
			"sort_field":   field,
			"tie_breakers": tieBreakers,
		}).
		Errorf("unable to tie-break sort, gotta have more tie-breakers")
	return 0
}
