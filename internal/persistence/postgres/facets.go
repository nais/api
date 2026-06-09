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
func (f *PostgresInstanceFacets) Environments(ctx context.Context) ([]*model.StringFacetItem, error) {
	filtered := f.Filtered(ctx)
	items := model.ComputeEnvironmentsFacet(f.AllInstances, filtered, func(inst *PostgresInstance) string {
		return inst.EnvironmentName
	})

	ret := make([]*model.StringFacetItem, len(items))
	for i := range items {
		ret[i] = &items[i]
	}
	return ret, nil
}

// States computes states facets for a Postgres query.
func (f *PostgresInstanceFacets) States(ctx context.Context) ([]*PostgresInstanceStateFacetItem, error) {
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

	ret := make([]*PostgresInstanceStateFacetItem, len(states))
	for i := range states {
		ret[i] = &states[i]
	}
	return ret, nil
}

// HighAvailability computes high availability facets for a Postgres query.
func (f *PostgresInstanceFacets) HighAvailability(ctx context.Context) ([]*model.BooleanFacetItem, error) {
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

	ret := make([]*model.BooleanFacetItem, len(ha))
	for i := range ha {
		ret[i] = &ha[i]
	}
	return ret, nil
}

// MajorVersions computes major version facets for a Postgres query.
func (f *PostgresInstanceFacets) MajorVersions(ctx context.Context) ([]*model.StringFacetItem, error) {
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

	ret := make([]*model.StringFacetItem, len(versions))
	for i := range versions {
		ret[i] = &versions[i]
	}
	return ret, nil
}

// Labels computes labels facets for a Postgres query.
func (f *PostgresInstanceFacets) Labels(ctx context.Context) ([]*model.LabelFacetItem, error) {
	filtered := f.Filtered(ctx)
	items := model.ComputeLabelsFacet(f.AllInstances, filtered, func(inst *PostgresInstance) []*model.ResourceLabel {
		return inst.Labels
	})

	ret := make([]*model.LabelFacetItem, len(items))
	for i := range items {
		ret[i] = &items[i]
	}
	return ret, nil
}
