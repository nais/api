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

type LabelFacetItem struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	Count int    `json:"count"`
}

// SortLabelFacetItems sorts a slice of LabelFacetItem alphabetically by key, then value.
func SortLabelFacetItems(items []LabelFacetItem) {
	slices.SortFunc(items, func(a, b LabelFacetItem) int {
		if a.Key != b.Key {
			return strings.Compare(a.Key, b.Key)
		}
		return strings.Compare(a.Value, b.Value)
	})
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

// ComputeEnvironmentsFacet computes environments facets for any list of resources.
func ComputeEnvironmentsFacet[T any](all []T, filtered []T, getEnv func(T) string) []StringFacetItem {
	environmentCounts := map[string]int{}
	for _, item := range all {
		env := getEnv(item)
		if _, ok := environmentCounts[env]; !ok {
			environmentCounts[env] = 0
		}
	}

	for _, item := range filtered {
		env := getEnv(item)
		environmentCounts[env]++
	}

	environments := make([]StringFacetItem, 0, len(environmentCounts))
	for env, count := range environmentCounts {
		environments = append(environments, StringFacetItem{
			Value: env,
			Count: count,
		})
	}
	SortStringFacetItems(environments)
	return environments
}

// ComputeLabelsFacet computes labels facets for any list of resources.
func ComputeLabelsFacet[T any](all []T, filtered []T, getLabels func(T) []*ResourceLabel) []LabelFacetItem {
	labelCounts := map[string]map[string]int{}
	for _, item := range all {
		for _, lbl := range getLabels(item) {
			if lbl == nil {
				continue
			}
			if _, ok := labelCounts[lbl.Key]; !ok {
				labelCounts[lbl.Key] = map[string]int{}
			}
			labelCounts[lbl.Key][lbl.Value] = 0
		}
	}

	for _, item := range filtered {
		for _, lbl := range getLabels(item) {
			if lbl == nil {
				continue
			}
			labelCounts[lbl.Key][lbl.Value]++
		}
	}

	labels := make([]LabelFacetItem, 0)
	for key, values := range labelCounts {
		for value, count := range values {
			labels = append(labels, LabelFacetItem{
				Key:   key,
				Value: value,
				Count: count,
			})
		}
	}
	SortLabelFacetItems(labels)
	return labels
}
