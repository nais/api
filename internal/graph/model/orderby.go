package model

import (
	"sort"

	"golang.org/x/exp/constraints"
)

func SortWith[T any](slice []T, eval func(a, b T) bool) {
	sort.SliceStable(slice, func(i, j int) bool {
		return eval(slice[i], slice[j])
	})
}

func Compare[T constraints.Ordered](a, b T, direction SortOrder) bool {
	if direction == SortOrderAsc {
		return a < b
	}
	return a > b
}
