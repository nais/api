package bigquery

import (
	"slices"
	"strings"

	"github.com/nais/api/internal/graph/model"
)

// ComputeFacets computes facets for a BigQuery dataset query.
// All possible values are seeded from allDatasets, but only items matching the filter are counted.
func ComputeFacets(allDatasets []*BigQueryDataset, filter *BigQueryDatasetFilter) *BigQueryDatasetFacets {
	// Seed all possible values from allDatasets
	environmentCounts := map[string]int{}

	for _, d := range allDatasets {
		environmentCounts[d.EnvironmentName] = 0
	}

	// Count only items matching the filter
	for _, d := range allDatasets {
		if !matchesFilter(d, filter) {
			continue
		}
		environmentCounts[d.EnvironmentName]++
	}

	return assembleFacets(environmentCounts)
}

// matchesFilter checks if a single dataset matches the given filter.
func matchesFilter(d *BigQueryDataset, filter *BigQueryDatasetFilter) bool {
	if filter == nil {
		return true
	}

	if filter.Name != "" {
		if !strings.Contains(strings.ToLower(d.Name), strings.ToLower(filter.Name)) {
			return false
		}
	}

	if len(filter.Environments) > 0 {
		if !slices.Contains(filter.Environments, d.EnvironmentName) {
			return false
		}
	}

	return true
}

func assembleFacets(environmentCounts map[string]int) *BigQueryDatasetFacets {
	facets := &BigQueryDatasetFacets{
		Environments: make([]model.EnvironmentFacetItem, 0, len(environmentCounts)),
	}

	for env, count := range environmentCounts {
		facets.Environments = append(facets.Environments, model.EnvironmentFacetItem{
			EnvironmentName: env,
			Count:           count,
		})
	}

	slices.SortFunc(facets.Environments, func(a, b model.EnvironmentFacetItem) int {
		return strings.Compare(a.EnvironmentName, b.EnvironmentName)
	})

	return facets
}
