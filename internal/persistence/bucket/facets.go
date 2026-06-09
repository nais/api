package bucket

import (
	"context"

	"github.com/nais/api/internal/graph/model"
)

// Filtered returns the filtered buckets, computing it exactly once per request.
func (f *BucketFacets) Filtered(ctx context.Context) []*Bucket {
	f.filteredOnce.Do(func() {
		f.filteredBuckets = SortFilter.Filter(ctx, f.AllBuckets, f.Filter)
	})
	return f.filteredBuckets
}

// Environments computes environments facets for a bucket query.
func (f *BucketFacets) Environments(ctx context.Context) []model.StringFacetItem {
	filtered := f.Filtered(ctx)
	return model.ComputeEnvironmentsFacet(f.AllBuckets, filtered, func(b *Bucket) string {
		return b.EnvironmentName
	})
}

// Labels computes labels facets for a bucket query.
func (f *BucketFacets) Labels(ctx context.Context) []model.LabelFacetItem {
	filtered := f.Filtered(ctx)
	return model.ComputeLabelsFacet(f.AllBuckets, filtered, func(b *Bucket) []*model.ResourceLabel {
		return b.Labels
	})
}
