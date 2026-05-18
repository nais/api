package bucket

import (
	"context"
	"slices"
	"strings"

	"github.com/nais/api/internal/graph/model"
)

func ComputeFacets(ctx context.Context, allBuckets []*Bucket, filter *BucketFilter) *BucketFacets {
	filtered := SortFilter.Filter(ctx, allBuckets, filter)

	// Seed all possible values from allBuckets
	environmentCounts := map[string]int{}

	for _, b := range allBuckets {
		environmentCounts[b.EnvironmentName] = 0
	}

	// Count only items matching the filter
	for _, b := range filtered {
		environmentCounts[b.EnvironmentName]++
	}

	return assembleFacets(environmentCounts)
}

func assembleFacets(environmentCounts map[string]int) *BucketFacets {
	facets := &BucketFacets{
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
