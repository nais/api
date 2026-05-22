package bigquery

import (
	"context"
	"slices"
	"strings"

	"github.com/nais/api/internal/graph/model"
)

func ComputeFacets(ctx context.Context, allDatasets []*BigQueryDataset, filter *BigQueryDatasetFilter) *BigQueryDatasetFacets {
	filtered := SortFilter.Filter(ctx, allDatasets, filter)

	// Seed all possible values from allDatasets
	environmentCounts := map[string]int{}

	for _, d := range allDatasets {
		environmentCounts[d.EnvironmentName] = 0
	}

	// Count only items matching the filter
	for _, d := range filtered {
		environmentCounts[d.EnvironmentName]++
	}

	return assembleFacets(environmentCounts)
}

func assembleFacets(environmentCounts map[string]int) *BigQueryDatasetFacets {
	facets := &BigQueryDatasetFacets{
		Environments: make([]model.StringFacetItem, 0, len(environmentCounts)),
	}

	for env, count := range environmentCounts {
		facets.Environments = append(facets.Environments, model.StringFacetItem{
			Value: env,
			Count: count,
		})
	}

	slices.SortFunc(facets.Environments, func(a, b model.StringFacetItem) int {
		return strings.Compare(a.Value, b.Value)
	})

	return facets
}
