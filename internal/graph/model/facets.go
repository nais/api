package model

import (
	"slices"
	"strings"
)

type BooleanFacetItem struct {
	Value bool `json:"value"`
	Count int  `json:"count"`
}

type StringFacetItem struct {
	Value string `json:"value"`
	Count int    `json:"count"`
}

// SortStringFacetItems sorts a slice of StringFacetItem alphabetically by value.
func SortStringFacetItems(items []StringFacetItem) {
	slices.SortFunc(items, func(a, b StringFacetItem) int {
		return strings.Compare(a.Value, b.Value)
	})
}

// SortBooleanFacetItems sorts a slice of BooleanFacetItem (false before true).
func SortBooleanFacetItems(items []BooleanFacetItem) {
	slices.SortFunc(items, func(a, b BooleanFacetItem) int {
		if a.Value == b.Value {
			return 0
		}
		if a.Value {
			return 1
		}
		return -1
	})
}
