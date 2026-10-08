package postgres

import (
	"context"
	"slices"
	"strings"

	"github.com/nais/api/internal/graph/model"
)

func (f *PostgresFacets) Labels(ctx context.Context) []model.LabelFacetItem {
	return model.ComputeLabelsFacet(f.AllInstances, f.FilteredInstances, func(pg *Postgres) []*model.ResourceLabel {
		return pg.Labels
	})
}

// Filtered returns the filtered Postgres instances, computing it exactly once per request.
func (f *PostgresBranchFacets) Filtered(ctx context.Context) []*PostgresBranch {
	f.filteredOnce.Do(func() {
		f.filteredInstances = SortFilterPostgresBranch.Filter(ctx, f.AllInstances, f.Filter)
	})
	return f.filteredInstances
}

// Environments computes environments facets for a Postgres query.
func (f *PostgresBranchFacets) Environments(ctx context.Context) []model.StringFacetItem {
	filtered := f.Filtered(ctx)
	return model.ComputeEnvironmentsFacet(f.AllInstances, filtered, func(inst *PostgresBranch) string {
		return inst.EnvironmentName
	})
}

// States computes states facets for a Postgres query.
func (f *PostgresBranchFacets) States(ctx context.Context) []PostgresBranchStateFacetItem {
	stateCounts := map[PostgresBranchState]int{}
	for _, inst := range f.AllInstances {
		stateCounts[inst.State] = 0
	}

	filtered := f.Filtered(ctx)
	for _, inst := range filtered {
		stateCounts[inst.State]++
	}

	states := make([]PostgresBranchStateFacetItem, 0, len(stateCounts))
	for state, count := range stateCounts {
		states = append(states, PostgresBranchStateFacetItem{
			State: state,
			Count: count,
		})
	}
	slices.SortFunc(states, func(a, b PostgresBranchStateFacetItem) int {
		return strings.Compare(a.State.String(), b.State.String())
	})

	return states
}

// Labels computes labels facets for a Postgres query.
func (f *PostgresBranchFacets) Labels(ctx context.Context) []model.LabelFacetItem {
	filtered := f.Filtered(ctx)
	return model.ComputeLabelsFacet(f.AllInstances, filtered, func(inst *PostgresBranch) []*model.ResourceLabel {
		return inst.Labels
	})
}
