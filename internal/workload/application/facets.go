package application

import (
	"context"
	"slices"
	"strings"

	"github.com/nais/api/internal/graph/model"
)

// Filtered returns the filtered applications, computing it exactly once per request.
func (f *ApplicationFacets) Filtered(ctx context.Context) []*Application {
	f.filteredOnce.Do(func() {
		f.filteredApps = SortFilter.Filter(ctx, f.AllApps, f.Filter)
	})
	return f.filteredApps
}

// Environments computes environments facets for an application query.
func (f *ApplicationFacets) Environments(ctx context.Context) []model.StringFacetItem {
	filtered := f.Filtered(ctx)
	return model.ComputeEnvironmentsFacet(f.AllApps, filtered, func(app *Application) string {
		return app.EnvironmentName
	})
}

// States computes states facets for an application query.
func (f *ApplicationFacets) States(ctx context.Context) []ApplicationStateFacetItem {
	stateCounts := map[ApplicationState]int{}
	for _, app := range f.AllApps {
		state, err := GetState(ctx, app)
		if err != nil {
			state = ApplicationStateUnknown
		}
		if _, ok := stateCounts[state]; !ok {
			stateCounts[state] = 0
		}
	}

	filtered := f.Filtered(ctx)
	for _, app := range filtered {
		state, err := GetState(ctx, app)
		if err != nil {
			state = ApplicationStateUnknown
		}
		stateCounts[state]++
	}

	states := make([]ApplicationStateFacetItem, 0, len(stateCounts))
	for state, count := range stateCounts {
		states = append(states, ApplicationStateFacetItem{
			State: state,
			Count: count,
		})
	}
	slices.SortFunc(states, func(a, b ApplicationStateFacetItem) int {
		return strings.Compare(a.State.String(), b.State.String())
	})

	return states
}

// Labels computes labels facets for an application query.
func (f *ApplicationFacets) Labels(ctx context.Context) []model.LabelFacetItem {
	filtered := f.Filtered(ctx)
	return model.ComputeLabelsFacet(f.AllApps, filtered, func(app *Application) []*model.ResourceLabel {
		return app.Labels
	})
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

	if !model.MatchesLabelFilters(app.Labels, filter.Labels) {
		return false
	}

	return true
}
