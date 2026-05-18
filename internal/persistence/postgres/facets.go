package postgres

import (
	"context"
	"slices"
	"strings"

	"github.com/nais/api/internal/graph/model"
)

func ComputeFacets(ctx context.Context, allInstances []*PostgresInstance, filter *PostgresInstanceFilter) *PostgresInstanceFacets {
	filtered := SortFilterPostgresInstance.Filter(ctx, allInstances, filter)

	// Seed all possible values from allInstances
	environmentCounts := map[string]int{}
	stateCounts := map[PostgresInstanceState]int{}
	haCounts := map[bool]int{}
	versionCounts := map[string]int{}

	for _, inst := range allInstances {
		environmentCounts[inst.EnvironmentName] = 0
		stateCounts[inst.State] = 0
		haCounts[inst.HighAvailability] = 0
		versionCounts[inst.MajorVersion] = 0
	}

	// Count only items matching the filter
	for _, inst := range filtered {
		environmentCounts[inst.EnvironmentName]++
		stateCounts[inst.State]++
		haCounts[inst.HighAvailability]++
		versionCounts[inst.MajorVersion]++
	}

	return assembleFacets(environmentCounts, stateCounts, haCounts, versionCounts)
}

func assembleFacets(
	environmentCounts map[string]int,
	stateCounts map[PostgresInstanceState]int,
	haCounts map[bool]int,
	versionCounts map[string]int,
) *PostgresInstanceFacets {
	facets := &PostgresInstanceFacets{
		Environments:     make([]model.EnvironmentFacetItem, 0, len(environmentCounts)),
		States:           make([]PostgresInstanceStateFacetItem, 0, len(stateCounts)),
		HighAvailability: make([]model.BooleanFacetItem, 0, len(haCounts)),
		MajorVersions:    make([]PostgresInstanceMajorVersionFacetItem, 0, len(versionCounts)),
	}

	for env, count := range environmentCounts {
		facets.Environments = append(facets.Environments, model.EnvironmentFacetItem{
			EnvironmentName: env,
			Count:           count,
		})
	}

	for state, count := range stateCounts {
		facets.States = append(facets.States, PostgresInstanceStateFacetItem{
			State: state,
			Count: count,
		})
	}

	for ha, count := range haCounts {
		facets.HighAvailability = append(facets.HighAvailability, model.BooleanFacetItem{
			Value: ha,
			Count: count,
		})
	}

	for version, count := range versionCounts {
		facets.MajorVersions = append(facets.MajorVersions, PostgresInstanceMajorVersionFacetItem{
			MajorVersion: version,
			Count:        count,
		})
	}

	// Sort for stable ordering
	slices.SortFunc(facets.Environments, func(a, b model.EnvironmentFacetItem) int {
		return strings.Compare(a.EnvironmentName, b.EnvironmentName)
	})

	slices.SortFunc(facets.States, func(a, b PostgresInstanceStateFacetItem) int {
		return strings.Compare(a.State.String(), b.State.String())
	})

	slices.SortFunc(facets.HighAvailability, func(a, b model.BooleanFacetItem) int {
		if a.Value == b.Value {
			return 0
		}
		if a.Value {
			return 1
		}
		return -1
	})

	slices.SortFunc(facets.MajorVersions, func(a, b PostgresInstanceMajorVersionFacetItem) int {
		return strings.Compare(a.MajorVersion, b.MajorVersion)
	})

	return facets
}
