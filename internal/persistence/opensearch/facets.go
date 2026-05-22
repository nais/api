package opensearch

import (
	"context"
	"slices"
	"strings"

	"github.com/nais/api/internal/graph/model"
)

func ComputeFacets(ctx context.Context, allInstances []*OpenSearch, filter *OpenSearchFilter) *OpenSearchFacets {
	filtered := SortFilterOpenSearch.Filter(ctx, allInstances, filter)

	environmentCounts := map[string]int{}
	tierCounts := map[OpenSearchTier]int{}

	for _, inst := range filtered {
		environmentCounts[inst.EnvironmentName]++
		tierCounts[inst.Tier]++
	}

	return assembleFacets(environmentCounts, tierCounts)
}

func assembleFacets(
	environmentCounts map[string]int,
	tierCounts map[OpenSearchTier]int,
) *OpenSearchFacets {
	facets := &OpenSearchFacets{
		Environments: make([]model.StringFacetItem, 0, len(environmentCounts)),
		Tiers:        make([]OpenSearchTierFacetItem, 0, len(tierCounts)),
	}

	for env, count := range environmentCounts {
		facets.Environments = append(facets.Environments, model.StringFacetItem{
			Value: env,
			Count: count,
		})
	}

	for tier, count := range tierCounts {
		facets.Tiers = append(facets.Tiers, OpenSearchTierFacetItem{
			Tier:  tier,
			Count: count,
		})
	}

	// Sort for stable ordering
	model.SortStringFacetItems(facets.Environments)

	slices.SortFunc(facets.Tiers, func(a, b OpenSearchTierFacetItem) int {
		return strings.Compare(a.Tier.String(), b.Tier.String())
	})

	return facets
}
