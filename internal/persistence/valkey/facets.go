package valkey

import (
	"slices"
	"strings"

	"github.com/nais/api/internal/graph/model"
)

// ComputeFacets computes facets for a Valkey query.
func ComputeFacets(allInstances []*Valkey, filter *ValkeyFilter) *ValkeyFacets {
	environmentCounts := map[string]int{}
	tierCounts := map[ValkeyTier]int{}

	for _, inst := range allInstances {
		if !matchesFilter(inst, filter) {
			continue
		}
		environmentCounts[inst.EnvironmentName]++
		tierCounts[inst.Tier]++
	}

	return assembleFacets(environmentCounts, tierCounts)
}

func matchesFilter(inst *Valkey, filter *ValkeyFilter) bool {
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
