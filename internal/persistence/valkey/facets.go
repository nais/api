package valkey

import (
	"context"
	"slices"
	"strings"

	"github.com/nais/api/internal/graph/model"
)

// Filtered returns the filtered Valkey instances, computing it exactly once per request.
func (f *ValkeyFacets) Filtered(ctx context.Context) []*Valkey {
	f.filteredOnce.Do(func() {
		f.filteredValkeys = SortFilterValkey.Filter(ctx, f.AllValkeys, f.Filter)
	})
	return f.filteredValkeys
}

// Environments computes environments facets for a Valkey query.
func (f *ValkeyFacets) Environments(ctx context.Context) []model.StringFacetItem {
	filtered := f.Filtered(ctx)
	return model.ComputeEnvironmentsFacet(f.AllValkeys, filtered, func(inst *Valkey) string {
		return inst.EnvironmentName
	})
}

// Tiers computes tiers facets for a Valkey query.
func (f *ValkeyFacets) Tiers(ctx context.Context) []ValkeyTierFacetItem {
	tierCounts := map[ValkeyTier]int{}
	for _, inst := range f.AllValkeys {
		tierCounts[inst.Tier] = 0
	}

	filtered := f.Filtered(ctx)
	for _, inst := range filtered {
		tierCounts[inst.Tier]++
	}

	tiers := make([]ValkeyTierFacetItem, 0, len(tierCounts))
	for tier, count := range tierCounts {
		tiers = append(tiers, ValkeyTierFacetItem{
			Tier:  tier,
			Count: count,
		})
	}
	slices.SortFunc(tiers, func(a, b ValkeyTierFacetItem) int {
		return strings.Compare(a.Tier.String(), b.Tier.String())
	})

	return tiers
}

// Labels computes labels facets for a Valkey query.
func (f *ValkeyFacets) Labels(ctx context.Context) []model.LabelFacetItem {
	filtered := f.Filtered(ctx)
	return model.ComputeLabelsFacet(f.AllValkeys, filtered, func(inst *Valkey) []*model.ResourceLabel {
		return inst.Labels
	})
}
