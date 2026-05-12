package application

import (
	"context"
	"slices"
	"strings"
)

// ComputeFacets computes facets for an application query. Called lazily by the
// resolver only when the client requests the facets field.
func ComputeFacets(ctx context.Context, allApps []*Application, filter *TeamApplicationsFilter) (*ApplicationFacets, error) {
	environmentCounts := map[string]int{}
	stateCounts := map[ApplicationState]int{}

	// First pass: seed with all values that exist in scope (so items with 0 matches still appear)
	for _, app := range allApps {
		if _, ok := environmentCounts[app.EnvironmentName]; !ok {
			environmentCounts[app.EnvironmentName] = 0
		}

		state, err := GetState(ctx, app)
		if err != nil {
			state = ApplicationStateUnknown
		}
		if _, ok := stateCounts[state]; !ok {
			stateCounts[state] = 0
		}
	}

	// Second pass: count apps that match the full filter
	for _, app := range allApps {
		if !matchesFilter(ctx, app, filter) {
			continue
		}

		environmentCounts[app.EnvironmentName]++

		state, err := GetState(ctx, app)
		if err != nil {
			state = ApplicationStateUnknown
		}
		stateCounts[state]++
	}

	return assembleFacets(environmentCounts, stateCounts), nil
}

func matchesFilter(ctx context.Context, app *Application, filter *TeamApplicationsFilter) bool {
	if filter == nil {
		return true
	}

	if filter.Name != "" {
		if !strings.Contains(strings.ToLower(app.Name), strings.ToLower(filter.Name)) {
			return false
		}
	}

	if len(filter.Environments) > 0 {
		if !slices.Contains(filter.Environments, app.EnvironmentName) {
			return false
		}
	}

	if len(filter.States) > 0 {
		state, err := GetState(ctx, app)
		if err != nil {
			if !slices.Contains(filter.States, ApplicationStateUnknown) {
				return false
			}
		} else if !slices.Contains(filter.States, state) {
			return false
		}
	}

	return true
}

func assembleFacets(environmentCounts map[string]int, stateCounts map[ApplicationState]int) *ApplicationFacets {
	facets := &ApplicationFacets{
		Environments: make([]ApplicationEnvironmentFacetItem, 0, len(environmentCounts)),
		States:       make([]ApplicationStateFacetItem, 0, len(stateCounts)),
	}

	for env, count := range environmentCounts {
		facets.Environments = append(facets.Environments, ApplicationEnvironmentFacetItem{
			EnvironmentName: env,
			Count:           count,
		})
	}

	for state, count := range stateCounts {
		facets.States = append(facets.States, ApplicationStateFacetItem{
			State: state,
			Count: count,
		})
	}

	// Sort alphabetically for stable ordering
	slices.SortFunc(facets.Environments, func(a, b ApplicationEnvironmentFacetItem) int {
		return strings.Compare(a.EnvironmentName, b.EnvironmentName)
	})

	slices.SortFunc(facets.States, func(a, b ApplicationStateFacetItem) int {
		return strings.Compare(a.State.String(), b.State.String())
	})

	return facets
}
