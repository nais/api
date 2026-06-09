package postgres

import (
	"context"
	"slices"
	"strings"

	"github.com/nais/api/internal/graph/model"
)

// Filtered returns the filtered Postgres instances, computing it exactly once per request.
func (f *PostgresInstanceFacets) Filtered(ctx context.Context) []*PostgresInstance {
	f.filteredOnce.Do(func() {
		f.filteredInstances = SortFilterPostgresInstance.Filter(ctx, f.AllInstances, f.Filter)
	})
	return f.filteredInstances
}

// Environments computes environments facets for a Postgres query.
func (f *PostgresInstanceFacets) Environments(ctx context.Context) []model.StringFacetItem {
	filtered := f.Filtered(ctx)
	return model.ComputeEnvironmentsFacet(f.AllInstances, filtered, func(inst *PostgresInstance) string {
		return inst.EnvironmentName
	})
}

// States computes states facets for a Postgres query.
func (f *PostgresInstanceFacets) States(ctx context.Context) []PostgresInstanceStateFacetItem {
	stateCounts := map[PostgresInstanceState]int{}
	for _, inst := range f.AllInstances {
		stateCounts[inst.State] = 0
	}

	filtered := f.Filtered(ctx)
	for _, inst := range filtered {
		stateCounts[inst.State]++
	}

	states := make([]PostgresInstanceStateFacetItem, 0, len(stateCounts))
	for state, count := range stateCounts {
		states = append(states, PostgresInstanceStateFacetItem{
			State: state,
			Count: count,
		})
	}
	slices.SortFunc(states, func(a, b PostgresInstanceStateFacetItem) int {
		return strings.Compare(a.State.String(), b.State.String())
	})

	return states
}

// HighAvailability computes high availability facets for a Postgres query.
func (f *PostgresInstanceFacets) HighAvailability(ctx context.Context) []model.BooleanFacetItem {
	haCounts := map[bool]int{}
	for _, inst := range f.AllInstances {
		haCounts[inst.HighAvailability] = 0
	}

	filtered := f.Filtered(ctx)
	for _, inst := range filtered {
		haCounts[inst.HighAvailability]++
	}

	ha := make([]model.BooleanFacetItem, 0, len(haCounts))
	for val, count := range haCounts {
		ha = append(ha, model.BooleanFacetItem{
			Value: val,
			Count: count,
		})
	}
	model.SortBooleanFacetItems(ha)

	return ha
}

// MajorVersions computes major version facets for a Postgres query.
func (f *PostgresInstanceFacets) MajorVersions(ctx context.Context) []model.StringFacetItem {
	versionCounts := map[string]int{}
	for _, inst := range f.AllInstances {
		versionCounts[inst.MajorVersion] = 0
	}

	filtered := f.Filtered(ctx)
	for _, inst := range filtered {
		versionCounts[inst.MajorVersion]++
	}

	versions := make([]model.StringFacetItem, 0, len(versionCounts))
	for val, count := range versionCounts {
		versions = append(versions, model.StringFacetItem{
			Value: val,
			Count: count,
		})
	}
	model.SortStringFacetItems(versions)

	return versions
}

// Labels computes labels facets for a Postgres query.
func (f *PostgresInstanceFacets) Labels(ctx context.Context) []model.LabelFacetItem {
	filtered := f.Filtered(ctx)
	return model.ComputeLabelsFacet(f.AllInstances, filtered, func(inst *PostgresInstance) []*model.ResourceLabel {
		return inst.Labels
	})
}
