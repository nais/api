package bucket

import (
	"slices"
	"strings"

	"github.com/nais/api/internal/graph/model"
)

// ComputeFacets computes facets for a bucket query.
// All possible values are seeded from allBuckets, but only items matching the filter are counted.
func ComputeFacets(allBuckets []*Bucket, filter *BucketFilter) *BucketFacets {
	// Seed all possible values from allBuckets
	environmentCounts := map[string]int{}

	for _, b := range allBuckets {
		environmentCounts[b.EnvironmentName] = 0
	}

	// Count only items matching the filter
	for _, b := range allBuckets {
		if !matchesFilter(b, filter) {
			continue
		}
		environmentCounts[b.EnvironmentName]++
	}

	return assembleFacets(environmentCounts)
}

// matchesFilter checks if a single bucket matches the given filter.
func matchesFilter(b *Bucket, filter *BucketFilter) bool {
	if filter == nil {
		return true
	}

	if filter.Name != "" {
		if !strings.Contains(strings.ToLower(b.Name), strings.ToLower(filter.Name)) {
			return false
		}
	}

	if len(filter.Environments) > 0 {
		if !slices.Contains(filter.Environments, b.EnvironmentName) {
			return false
		}
	}

	return true
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
