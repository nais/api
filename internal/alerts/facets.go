package alerts

import (
	"context"
	"slices"
	"strings"

	"github.com/nais/api/internal/graph/model"
)

func (f *AlertFacets) Filtered(ctx context.Context) []Alert {
	f.filteredOnce.Do(func() {
		f.filteredAlerts = SortFilter.Filter(ctx, f.AllAlerts, f.Filter)
	})
	return f.filteredAlerts
}

func (f *AlertFacets) Environments(ctx context.Context) []model.StringFacetItem {
	filtered := f.Filtered(ctx)
	return model.ComputeEnvironmentsFacet(f.AllAlerts, filtered, func(a Alert) string {
		return a.GetEnvironmentName()
	})
}

func (f *AlertFacets) States(ctx context.Context) []AlertStateFacetItem {
	stateCounts := map[AlertState]int{}
	for _, a := range f.AllAlerts {
		if _, ok := stateCounts[a.GetState()]; !ok {
			stateCounts[a.GetState()] = 0
		}
	}
	for _, a := range f.Filtered(ctx) {
		stateCounts[a.GetState()]++
	}

	items := make([]AlertStateFacetItem, 0, len(stateCounts))
	for state, count := range stateCounts {
		items = append(items, AlertStateFacetItem{State: state, Count: count})
	}
	slices.SortFunc(items, func(a, b AlertStateFacetItem) int {
		return strings.Compare(a.State.String(), b.State.String())
	})
	return items
}
