package sortfilter

// TODO(thokra): Some filters and orderbys is probably slow to run for each item,
// consider doing a call for each element first, especially in Sort, and then return
// the result.

import (
	"context"
	"fmt"
	"slices"

	"github.com/nais/api/internal/v1/graphv1/modelv1"
)

// OrderBy compares two values of type V and returns an integer indicating their order.
// If a < b, the function should return a negative value.
// If a == b, the function should return 0.
// If a > b, the function should return a positive value.
type OrderBy[V any] func(ctx context.Context, a, b V) int

type Filter[V any, FilterObj any] func(ctx context.Context, v V, filter FilterObj) bool

type SortFilter[V any, OrderKey comparable, FilterObj comparable] struct {
	orderBys map[OrderKey]OrderBy[V]
	filters  []Filter[V, FilterObj]
}

func New[V any, OrderKey comparable, FilterObj comparable]() *SortFilter[V, OrderKey, FilterObj] {
	return &SortFilter[V, OrderKey, FilterObj]{
		orderBys: make(map[OrderKey]OrderBy[V]),
	}
}

func (s *SortFilter[T, OrderKey, FilterObj]) RegisterFilter(filter Filter[T, FilterObj]) {
	s.filters = append(s.filters, filter)
}

func (s *SortFilter[T, OrderKey, FilterObj]) RegisterOrderBy(key OrderKey, orderBy OrderBy[T]) {
	if _, ok := s.orderBys[key]; ok {
		panic(fmt.Sprintf("OrderBy already registered for key: %v", key))
	}
	s.orderBys[key] = orderBy
}

func (s *SortFilter[T, OrderKey, FilterObj]) Filter(ctx context.Context, items []T, filter FilterObj) []T {
	var nillish FilterObj
	if filter == nillish {
		return items
	}

	filtered := make([]T, 0, len(items))
	for _, item := range items {
		include := true
		for _, fn := range s.filters {
			if !fn(ctx, item, filter) {
				include = false
				break
			}
		}
		if include {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func (s *SortFilter[T, OrderKey, FilterObj]) Sort(ctx context.Context, items []T, key OrderKey, direction modelv1.OrderDirection) {
	orderBy, ok := s.orderBys[key]
	if !ok {
		panic(fmt.Sprintf("OrderBy not registered for key: %v", key))
	}

	slices.SortStableFunc(items, func(i, j T) int {
		if direction == modelv1.OrderDirectionDesc {
			return orderBy(ctx, j, i)
		}
		return orderBy(ctx, i, j)
	})
}
