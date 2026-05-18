package postgres

import (
	"slices"
	"strings"

	"github.com/nais/api/internal/graph/model"
)

// ComputeFacets computes facets for a Postgres instance query.
// All possible values are seeded from allInstances, but only items matching the filter are counted.
func ComputeFacets(allInstances []*PostgresInstance, filter *PostgresInstanceFilter) *PostgresInstanceFacets {
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
	for _, inst := range allInstances {
		if !matchesFilter(inst, filter) {
			continue
		}
		environmentCounts[inst.EnvironmentName]++
		stateCounts[inst.State]++
		haCounts[inst.HighAvailability]++
		versionCounts[inst.MajorVersion]++
	}

	return assembleFacets(environmentCounts, stateCounts, haCounts, versionCounts)
}

// matchesFilter checks if a single instance matches the given filter.
func matchesFilter(inst *PostgresInstance, filter *PostgresInstanceFilter) bool {
	if filter == nil {
		return true
	}

	if filter.Name != "" {
		if !strings.Contains(strings.ToLower(inst.Name), strings.ToLower(filter.Name)) {
			return false
		}
	}

	if len(filter.Environments) > 0 {
		if !slices.Contains(filter.Environments, inst.EnvironmentName) {
			return false
		}
	}

	if len(filter.States) > 0 {
		if !slices.Contains(filter.States, inst.State) {
			return false
		}
	}

	if filter.HighAvailability != nil {
		if inst.HighAvailability != *filter.HighAvailability {
			return false
		}
	}

	if len(filter.MajorVersions) > 0 {
		if !slices.Contains(filter.MajorVersions, inst.MajorVersion) {
			return false
		}
	}

	return true
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
