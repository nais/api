package alerts

import (
	"context"
	"reflect"
	"testing"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/slug"
)

func makeAlert(name, env string, state AlertState) Alert {
	return &PrometheusAlert{
		BaseAlert: BaseAlert{
			Name:            name,
			State:           state,
			TeamSlug:        slug.Slug("myteam"),
			EnvironmentName: env,
		},
	}
}

func TestAlertFacets_Environments(t *testing.T) {
	ctx := context.Background()

	all := []Alert{
		makeAlert("a1", "prod", AlertStateFiring),
		makeAlert("a2", "prod", AlertStateInactive),
		makeAlert("a3", "dev", AlertStateFiring),
		makeAlert("a4", "dev", AlertStatePending),
	}

	tests := []struct {
		name   string
		filter *TeamAlertsFilter
		want   []model.StringFacetItem
	}{
		{
			name:   "no filter: all counts match totals",
			filter: nil,
			want: []model.StringFacetItem{
				{Value: "dev", Count: 2},
				{Value: "prod", Count: 2},
			},
		},
		{
			name:   "filter by environment: other env count is zero but still present",
			filter: &TeamAlertsFilter{Environments: []string{"prod"}},
			want: []model.StringFacetItem{
				{Value: "dev", Count: 0},
				{Value: "prod", Count: 2},
			},
		},
		{
			name:   "filter by state: env counts reflect matching alerts",
			filter: &TeamAlertsFilter{States: []AlertState{AlertStateFiring}},
			want: []model.StringFacetItem{
				{Value: "dev", Count: 1},
				{Value: "prod", Count: 1},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &AlertFacets{AllAlerts: all, Filter: tt.filter}
			got := f.Environments(ctx)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Environments =\n  %v\nwant\n  %v", got, tt.want)
			}
		})
	}
}

func TestAlertFacets_States(t *testing.T) {
	ctx := context.Background()

	all := []Alert{
		makeAlert("a1", "prod", AlertStateFiring),
		makeAlert("a2", "prod", AlertStateInactive),
		makeAlert("a3", "dev", AlertStateFiring),
		makeAlert("a4", "dev", AlertStatePending),
	}

	tests := []struct {
		name   string
		filter *TeamAlertsFilter
		want   []AlertStateFacetItem
	}{
		{
			name:   "no filter: all states present with full counts",
			filter: nil,
			want: []AlertStateFacetItem{
				{State: AlertStateFiring, Count: 2},
				{State: AlertStateInactive, Count: 1},
				{State: AlertStatePending, Count: 1},
			},
		},
		{
			name:   "filter by state: non-matching states have count 0",
			filter: &TeamAlertsFilter{States: []AlertState{AlertStateFiring}},
			want: []AlertStateFacetItem{
				{State: AlertStateFiring, Count: 2},
				{State: AlertStateInactive, Count: 0},
				{State: AlertStatePending, Count: 0},
			},
		},
		{
			name:   "filter by environment: state counts reflect only matching env",
			filter: &TeamAlertsFilter{Environments: []string{"prod"}},
			want: []AlertStateFacetItem{
				{State: AlertStateFiring, Count: 1},
				{State: AlertStateInactive, Count: 1},
				{State: AlertStatePending, Count: 0},
			},
		},
		{
			name:   "empty alerts: no facet items",
			filter: nil,
			want:   []AlertStateFacetItem{},
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alertsSlice := all
			if i == len(tests)-1 {
				alertsSlice = nil
			}
			f := &AlertFacets{AllAlerts: alertsSlice, Filter: tt.filter}
			got := f.States(ctx)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("States =\n  %v\nwant\n  %v", got, tt.want)
			}
		})
	}
}
