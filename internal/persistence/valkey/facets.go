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
func (f *ValkeyFacets) Environments(ctx context.Context) ([]*model.StringFacetItem, error) {
	filtered := f.Filtered(ctx)
	items := model.ComputeEnvironmentsFacet(f.AllValkeys, filtered, func(inst *Valkey) string {
		return inst.EnvironmentName
	})

	ret := make([]*model.StringFacetItem, len(items))
	for i := range items {
		ret[i] = &items[i]
	}
	return ret, nil
}

// Tiers computes tiers facets for a Valkey query.
func (f *ValkeyFacets) Tiers(ctx context.Context) ([]*ValkeyTierFacetItem, error) {
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

	ret := make([]*ValkeyTierFacetItem, len(tiers))
	for i := range tiers {
		ret[i] = &tiers[i]
	}
	return ret, nil
}

// Labels computes labels facets for a Valkey query.
func (f *ValkeyFacets) Labels(ctx context.Context) ([]*model.LabelFacetItem, error) {
	filtered := f.Filtered(ctx)
	items := model.ComputeLabelsFacet(f.AllValkeys, filtered, func(inst *Valkey) []*model.ResourceLabel {
		return inst.Labels
	})

	ret := make([]*model.LabelFacetItem, len(items))
	for i := range items {
		ret[i] = &items[i]
	}
	return ret, nil
}
