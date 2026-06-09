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
func (f *BigQueryDatasetFacets) Environments(ctx context.Context) []model.StringFacetItem {
	filtered := f.Filtered(ctx)
	return model.ComputeEnvironmentsFacet(f.AllDatasets, filtered, func(d *BigQueryDataset) string {
		return d.EnvironmentName
	})
}

// Labels computes labels facets for a BigQuery dataset query.
func (f *BigQueryDatasetFacets) Labels(ctx context.Context) []model.LabelFacetItem {
	filtered := f.Filtered(ctx)
	return model.ComputeLabelsFacet(f.AllDatasets, filtered, func(d *BigQueryDataset) []*model.ResourceLabel {
		return d.Labels
	})
}
