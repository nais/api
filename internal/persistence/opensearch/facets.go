package opensearch

import (
	"context"
	"slices"
	"strings"

	"github.com/nais/api/internal/graph/model"
)

// Filtered returns the filtered OpenSearch instances, computing it exactly once per request.
func (f *OpenSearchFacets) Filtered(ctx context.Context) []*OpenSearch {
	f.filteredOnce.Do(func() {
		f.filteredOpenSearches = SortFilterOpenSearch.Filter(ctx, f.AllOpenSearches, f.Filter)
	})
	return f.filteredOpenSearches
}

// Environments computes environments facets for an OpenSearch query.
func (f *OpenSearchFacets) Environments(ctx context.Context) ([]*model.StringFacetItem, error) {
	filtered := f.Filtered(ctx)
	items := model.ComputeEnvironmentsFacet(f.AllOpenSearches, filtered, func(inst *OpenSearch) string {
		return inst.EnvironmentName
	})

	ret := make([]*model.StringFacetItem, len(items))
	for i := range items {
		ret[i] = &items[i]
	}
	return ret, nil
}

// Tiers computes tiers facets for an OpenSearch query.
func (f *OpenSearchFacets) Tiers(ctx context.Context) ([]*OpenSearchTierFacetItem, error) {
	tierCounts := map[OpenSearchTier]int{}
	for _, inst := range f.AllOpenSearches {
		tierCounts[inst.Tier] = 0
	}

	filtered := f.Filtered(ctx)
	for _, inst := range filtered {
		tierCounts[inst.Tier]++
	}

	tiers := make([]OpenSearchTierFacetItem, 0, len(tierCounts))
	for tier, count := range tierCounts {
		tiers = append(tiers, OpenSearchTierFacetItem{
			Tier:  tier,
			Count: count,
		})
	}
	slices.SortFunc(tiers, func(a, b OpenSearchTierFacetItem) int {
		return strings.Compare(a.Tier.String(), b.Tier.String())
	})

	ret := make([]*OpenSearchTierFacetItem, len(tiers))
	for i := range tiers {
		ret[i] = &tiers[i]
	}
	return ret, nil
}

// Labels computes labels facets for an OpenSearch query.
func (f *OpenSearchFacets) Labels(ctx context.Context) ([]*model.LabelFacetItem, error) {
	filtered := f.Filtered(ctx)
	items := model.ComputeLabelsFacet(f.AllOpenSearches, filtered, func(inst *OpenSearch) []*model.ResourceLabel {
		return inst.Labels
	})

	ret := make([]*model.LabelFacetItem, len(items))
	for i := range items {
		ret[i] = &items[i]
	}
	return ret, nil
}
