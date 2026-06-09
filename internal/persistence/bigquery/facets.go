package bigquery

import (
	"context"

	"github.com/nais/api/internal/graph/model"
)

// Filtered returns the filtered BigQuery datasets, computing it exactly once per request.
func (f *BigQueryDatasetFacets) Filtered(ctx context.Context) []*BigQueryDataset {
	f.filteredOnce.Do(func() {
		f.filteredDatasets = SortFilter.Filter(ctx, f.AllDatasets, f.Filter)
	})
	return f.filteredDatasets
}

// Environments computes environments facets for a BigQuery dataset query.
func (f *BigQueryDatasetFacets) Environments(ctx context.Context) ([]*model.StringFacetItem, error) {
	filtered := f.Filtered(ctx)
	items := model.ComputeEnvironmentsFacet(f.AllDatasets, filtered, func(d *BigQueryDataset) string {
		return d.EnvironmentName
	})

	ret := make([]*model.StringFacetItem, len(items))
	for i := range items {
		ret[i] = &items[i]
	}
	return ret, nil
}

// Labels computes labels facets for a BigQuery dataset query.
func (f *BigQueryDatasetFacets) Labels(ctx context.Context) ([]*model.LabelFacetItem, error) {
	filtered := f.Filtered(ctx)
	items := model.ComputeLabelsFacet(f.AllDatasets, filtered, func(d *BigQueryDataset) []*model.ResourceLabel {
		return d.Labels
	})

	ret := make([]*model.LabelFacetItem, len(items))
	for i := range items {
		ret[i] = &items[i]
	}
	return ret, nil
}
