package valkey

import (
	"context"
	"slices"
	"strings"

	"github.com/nais/api/internal/graph/model"
)

func ComputeFacets(ctx context.Context, allInstances []*Valkey, filter *ValkeyFilter) *ValkeyFacets {
	filtered := SortFilterValkey.Filter(ctx, allInstances, filter)

	environmentCounts := map[string]int{}
	tierCounts := map[ValkeyTier]int{}

	for _, inst := range filtered {
		environmentCounts[inst.EnvironmentName]++
		tierCounts[inst.Tier]++
	}

	return assembleFacets(environmentCounts, tierCounts)
}

func assembleFacets(
	environmentCounts map[string]int,
	tierCounts map[ValkeyTier]int,
) *ValkeyFacets {
	facets := &ValkeyFacets{
		Environments: make([]model.EnvironmentFacetItem, 0, len(environmentCounts)),
		Tiers:        make([]ValkeyTierFacetItem, 0, len(tierCounts)),
	}

	for env, count := range environmentCounts {
		facets.Environments = append(facets.Environments, model.EnvironmentFacetItem{
			EnvironmentName: env,
			Count:           count,
		})
	}

	for tier, count := range tierCounts {
		facets.Tiers = append(facets.Tiers, ValkeyTierFacetItem{
			Tier:  tier,
			Count: count,
		})
	}

	// Sort for stable ordering
	slices.SortFunc(facets.Environments, func(a, b model.EnvironmentFacetItem) int {
		return strings.Compare(a.EnvironmentName, b.EnvironmentName)
	})

	slices.SortFunc(facets.Tiers, func(a, b ValkeyTierFacetItem) int {
		return strings.Compare(a.Tier.String(), b.Tier.String())
	})

	return facets
}
