package postgres

import (
	"context"

	"github.com/nais/api/internal/graph/model"
)

func (f *PostgresFacets) Labels(ctx context.Context) []model.LabelFacetItem {
	return model.ComputeLabelsFacet(f.AllInstances, f.FilteredInstances, func(pg *Postgres) []*model.ResourceLabel {
		return pg.Labels
	})
}
