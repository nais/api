package sortfilter

// TODO(thokra): Some filters and orderbys is probably slow to run for each item,
// consider doing a call for each element first, especially in Sort, and then return
// the result.

import (
	"context"
	"fmt"
	"slices"

	"github.com/nais/api/internal/v1/graphv1/modelv1"
	"github.com/sourcegraph/conc/pool"
)

// OrderBy compares two values of type V and returns an integer indicating their order.
// If a < b, the function should return a negative value.
// If a == b, the function should return 0.
// If a > b, the function should return a positive value.
type OrderBy[V any] func(ctx context.Context, a, b V) int

// ConcurrentOrderBy should return a integer indicating the order of the given value.
// The results will later be ordered by the returned value.
type ConcurrentOrderBy[V any] func(ctx context.Context, a V) int

type Filter[V any, FilterObj any] func(ctx context.Context, v V, filter FilterObj) bool

type orderByValue[V any] struct {
	concurrentOrderBy ConcurrentOrderBy[V]
	orderBy           OrderBy[V]
}

type SortFilter[V any, OrderKey comparable, FilterObj comparable] struct {
	orderBys       map[OrderKey]orderByValue[V]
	filters        []Filter[V, FilterObj]
	defaultSortKey OrderKey
}

// New creates a new SortFilter with the given defaultSortKey.
// The defaultSortKey is used when two values are equal in the OrderBy function.
// The defaultSortKey must not be registered as a ConcurrentOrderBy.
func New[V any, OrderKey comparable, FilterObj comparable](defaultSortKey OrderKey) *SortFilter[V, OrderKey, FilterObj] {
	return &SortFilter[V, OrderKey, FilterObj]{
		orderBys:       make(map[OrderKey]orderByValue[V]),
		defaultSortKey: defaultSortKey,
	}
}

func (s *SortFilter[T, OrderKey, FilterObj]) RegisterFilter(filter Filter[T, FilterObj]) {
	s.filters = append(s.filters, filter)
}

func (s *SortFilter[T, OrderKey, FilterObj]) RegisterOrderBy(key OrderKey, orderBy OrderBy[T]) {
	if _, ok := s.orderBys[key]; ok {
		panic(fmt.Sprintf("OrderBy already registered for key: %v", key))
	}
	s.orderBys[key] = orderByValue[T]{
		orderBy: orderBy,
	}
}

func (s *SortFilter[T, OrderKey, FilterObj]) RegisterConcurrentOrderBy(key OrderKey, orderBy ConcurrentOrderBy[T]) {
	if _, ok := s.orderBys[key]; ok {
		panic(fmt.Sprintf("OrderBy already registered for key: %v", key))
	}
	s.orderBys[key] = orderByValue[T]{
		concurrentOrderBy: orderBy,
	}
}

func (s *SortFilter[T, OrderKey, FilterObj]) Filter(ctx context.Context, items []T, filter FilterObj) []T {
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

func (s *SortFilter[T, OrderKey, FilterObj]) Sort(ctx context.Context, items []T, key OrderKey, direction modelv1.OrderDirection) {
	orderBy, ok := s.orderBys[key]
	if !ok {
		panic(fmt.Sprintf("OrderBy not registered for key: %v", key))
	}

	if len(items) == 0 {
		return
	}

	if orderBy.concurrentOrderBy != nil {
		s.sortConcurrent(ctx, items, orderBy.concurrentOrderBy, direction)
		return
	}

	s.sort(ctx, items, orderBy.orderBy, direction)
}

func (s *SortFilter[T, OrderKey, FilterObj]) sortConcurrent(ctx context.Context, items []T, orderBy ConcurrentOrderBy[T], direction modelv1.OrderDirection) {
	type sortable struct {
		item T
		key  int
	}

	wg := pool.NewWithResults[sortable]().WithMaxGoroutines(50).WithContext(ctx)
	for _, item := range items {
		wg.Go(func(ctx context.Context) (sortable, error) {
			return sortable{
				item: item,
				key:  orderBy(ctx, item),
			}, nil
		})
	}

	res, err := wg.Wait()
	if err != nil {
		// This should never happen, as orderBy doesn't return errors.
		panic(err)
	}

	slices.SortStableFunc(res, func(i, j sortable) int {
		if j.key == i.key {
			return s.defaultSort(ctx, i.item, j.item, direction)
		}

		if direction == modelv1.OrderDirectionDesc {
			return j.key - i.key
		}
		return i.key - j.key
	})

	for i, r := range res {
		items[i] = r.item
	}
}

func (s *SortFilter[T, OrderKey, FilterObj]) sort(ctx context.Context, items []T, orderBy OrderBy[T], direction modelv1.OrderDirection) {
	slices.SortStableFunc(items, func(i, j T) int {
		var ret int
		if direction == modelv1.OrderDirectionDesc {
			ret = orderBy(ctx, j, i)
		} else {
			ret = orderBy(ctx, i, j)
		}

		if ret == 0 {
			return s.defaultSort(ctx, i, j, direction)
		}
		return ret
	})
}

func (s *SortFilter[T, OrderKey, FilterObj]) defaultSort(ctx context.Context, a, b T, direction modelv1.OrderDirection) int {
	if direction == modelv1.OrderDirectionDesc {
		return s.orderBys[s.defaultSortKey].orderBy(ctx, b, a)
	}
	return s.orderBys[s.defaultSortKey].orderBy(ctx, a, b)
}
