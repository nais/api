package opensearch

import (
	"slices"
	"strings"

	"github.com/nais/api/internal/graph/model"
)

// ComputeFacets computes facets for an OpenSearch query.
func ComputeFacets(allInstances []*OpenSearch, filter *OpenSearchFilter) *OpenSearchFacets {
	environmentCounts := map[string]int{}
	tierCounts := map[OpenSearchTier]int{}

	for _, inst := range allInstances {
		if !matchesFilter(inst, filter) {
			continue
		}
		environmentCounts[inst.EnvironmentName]++
		tierCounts[inst.Tier]++
	}

	return assembleFacets(environmentCounts, tierCounts)
}

func matchesFilter(inst *OpenSearch, filter *OpenSearchFilter) bool {
	if filter == nil {
		return true
	}

	if filter.Name != "" {
		if !strings.Contains(strings.ToLower(inst.Name), strings.ToLower(filter.Name)) {
			return false
		}
	}

	if len(filter.Environments) > 0 {
		if !slices.Contains(filter.Environments, inst.EnvironmentName) {
			return false
		}
	}

	if len(filter.Tiers) > 0 {
		if !slices.Contains(filter.Tiers, inst.Tier) {
			return false
		}
	}

	return true
}

func assembleFacets(
	environmentCounts map[string]int,
	tierCounts map[OpenSearchTier]int,
) *OpenSearchFacets {
	facets := &OpenSearchFacets{
		Environments: make([]model.EnvironmentFacetItem, 0, len(environmentCounts)),
		Tiers:        make([]OpenSearchTierFacetItem, 0, len(tierCounts)),
	}

	for env, count := range environmentCounts {
		facets.Environments = append(facets.Environments, model.EnvironmentFacetItem{
			EnvironmentName: env,
			Count:           count,
		})
	}

	for tier, count := range tierCounts {
		facets.Tiers = append(facets.Tiers, OpenSearchTierFacetItem{
			Tier:  tier,
			Count: count,
		})
	}

	// Sort for stable ordering
	slices.SortFunc(facets.Environments, func(a, b model.EnvironmentFacetItem) int {
		return strings.Compare(a.EnvironmentName, b.EnvironmentName)
	})

	slices.SortFunc(facets.Tiers, func(a, b OpenSearchTierFacetItem) int {
		return strings.Compare(a.Tier.String(), b.Tier.String())
	})

	return facets
}
