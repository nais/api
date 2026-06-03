package postgres

import (
	"context"
	"reflect"
	"testing"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/slug"
)

func TestComputeFacets(t *testing.T) {
	boolPtr := func(v bool) *bool { return &v }

	allInstances := []*PostgresInstance{
		{
			Name:             "app-db-1",
			EnvironmentName:  "dev",
			TeamSlug:         slug.Slug("my-team"),
			MajorVersion:     "15",
			HighAvailability: false,
			State:            PostgresInstanceStateAvailable,
		},
		{
			Name:             "app-db-2",
			EnvironmentName:  "dev",
			TeamSlug:         slug.Slug("my-team"),
			MajorVersion:     "16",
			HighAvailability: true,
			State:            PostgresInstanceStateProgressing,
		},
		{
			Name:             "app-db-3",
			EnvironmentName:  "prod",
			TeamSlug:         slug.Slug("my-team"),
			MajorVersion:     "15",
			HighAvailability: true,
			State:            PostgresInstanceStateAvailable,
		},
		{
			Name:             "app-db-4",
			EnvironmentName:  "prod",
			TeamSlug:         slug.Slug("my-team"),
			MajorVersion:     "17",
			HighAvailability: false,
			State:            PostgresInstanceStateDegraded,
		},
	}

	tests := []struct {
		name              string
		instances         []*PostgresInstance
		filter            *PostgresInstanceFilter
		wantEnvironments  []model.StringFacetItem
		wantStates        []PostgresInstanceStateFacetItem
		wantHA            []model.BooleanFacetItem
		wantMajorVersions []model.StringFacetItem
	}{
		{
			name:      "no filter counts all instances",
			instances: allInstances,
			filter:    nil,
			wantEnvironments: []model.StringFacetItem{
				{Value: "dev", Count: 2},
				{Value: "prod", Count: 2},
			},
			wantStates: []PostgresInstanceStateFacetItem{
				{State: PostgresInstanceStateAvailable, Count: 2},
				{State: PostgresInstanceStateDegraded, Count: 1},
				{State: PostgresInstanceStateProgressing, Count: 1},
			},
			wantHA: []model.BooleanFacetItem{
				{Value: false, Count: 2},
				{Value: true, Count: 2},
			},
			wantMajorVersions: []model.StringFacetItem{
				{Value: "15", Count: 2},
				{Value: "16", Count: 1},
				{Value: "17", Count: 1},
			},
		},
		{
			name:      "filter by environment counts only matching but seeds all",
			instances: allInstances,
			filter:    &PostgresInstanceFilter{Environments: []string{"dev"}},
			wantEnvironments: []model.StringFacetItem{
				{Value: "dev", Count: 2},
				{Value: "prod", Count: 0},
			},
			wantStates: []PostgresInstanceStateFacetItem{
				{State: PostgresInstanceStateAvailable, Count: 1},
				{State: PostgresInstanceStateDegraded, Count: 0},
				{State: PostgresInstanceStateProgressing, Count: 1},
			},
			wantHA: []model.BooleanFacetItem{
				{Value: false, Count: 1},
				{Value: true, Count: 1},
			},
			wantMajorVersions: []model.StringFacetItem{
				{Value: "15", Count: 1},
				{Value: "16", Count: 1},
				{Value: "17", Count: 0},
			},
		},
		{
			name:      "filter by state counts only matching state",
			instances: allInstances,
			filter:    &PostgresInstanceFilter{States: []PostgresInstanceState{PostgresInstanceStateAvailable}},
			wantEnvironments: []model.StringFacetItem{
				{Value: "dev", Count: 1},
				{Value: "prod", Count: 1},
			},
			wantStates: []PostgresInstanceStateFacetItem{
				{State: PostgresInstanceStateAvailable, Count: 2},
				{State: PostgresInstanceStateDegraded, Count: 0},
				{State: PostgresInstanceStateProgressing, Count: 0},
			},
			wantHA: []model.BooleanFacetItem{
				{Value: false, Count: 1},
				{Value: true, Count: 1},
			},
			wantMajorVersions: []model.StringFacetItem{
				{Value: "15", Count: 2},
				{Value: "16", Count: 0},
				{Value: "17", Count: 0},
			},
		},
		{
			name:      "filter by high availability",
			instances: allInstances,
			filter:    &PostgresInstanceFilter{HighAvailability: boolPtr(true)},
			wantEnvironments: []model.StringFacetItem{
				{Value: "dev", Count: 1},
				{Value: "prod", Count: 1},
			},
			wantStates: []PostgresInstanceStateFacetItem{
				{State: PostgresInstanceStateAvailable, Count: 1},
				{State: PostgresInstanceStateDegraded, Count: 0},
				{State: PostgresInstanceStateProgressing, Count: 1},
			},
			wantHA: []model.BooleanFacetItem{
				{Value: false, Count: 0},
				{Value: true, Count: 2},
			},
			wantMajorVersions: []model.StringFacetItem{
				{Value: "15", Count: 1},
				{Value: "16", Count: 1},
				{Value: "17", Count: 0},
			},
		},
		{
			name:      "filter by major version",
			instances: allInstances,
			filter:    &PostgresInstanceFilter{MajorVersions: []string{"15"}},
			wantEnvironments: []model.StringFacetItem{
				{Value: "dev", Count: 1},
				{Value: "prod", Count: 1},
			},
			wantStates: []PostgresInstanceStateFacetItem{
				{State: PostgresInstanceStateAvailable, Count: 2},
				{State: PostgresInstanceStateDegraded, Count: 0},
				{State: PostgresInstanceStateProgressing, Count: 0},
			},
			wantHA: []model.BooleanFacetItem{
				{Value: false, Count: 1},
				{Value: true, Count: 1},
			},
			wantMajorVersions: []model.StringFacetItem{
				{Value: "15", Count: 2},
				{Value: "16", Count: 0},
				{Value: "17", Count: 0},
			},
		},
		{
			name:      "combined filter environment and state",
			instances: allInstances,
			filter: &PostgresInstanceFilter{
				Environments: []string{"prod"},
				States:       []PostgresInstanceState{PostgresInstanceStateAvailable},
			},
			wantEnvironments: []model.StringFacetItem{
				{Value: "dev", Count: 0},
				{Value: "prod", Count: 1},
			},
			wantStates: []PostgresInstanceStateFacetItem{
				{State: PostgresInstanceStateAvailable, Count: 1},
				{State: PostgresInstanceStateDegraded, Count: 0},
				{State: PostgresInstanceStateProgressing, Count: 0},
			},
			wantHA: []model.BooleanFacetItem{
				{Value: false, Count: 0},
				{Value: true, Count: 1},
			},
			wantMajorVersions: []model.StringFacetItem{
				{Value: "15", Count: 1},
				{Value: "16", Count: 0},
				{Value: "17", Count: 0},
			},
		},
		{
			name:              "empty input returns empty facets",
			instances:         nil,
			filter:            nil,
			wantEnvironments:  []model.StringFacetItem{},
			wantStates:        []PostgresInstanceStateFacetItem{},
			wantHA:            []model.BooleanFacetItem{},
			wantMajorVersions: []model.StringFacetItem{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			got := ComputeFacets(ctx, tt.instances, tt.filter)

			if !reflect.DeepEqual(got.Environments, tt.wantEnvironments) {
				t.Errorf("Environments =\n  %v\nwant\n  %v", got.Environments, tt.wantEnvironments)
			}

			if !reflect.DeepEqual(got.States, tt.wantStates) {
				t.Errorf("States =\n  %v\nwant\n  %v", got.States, tt.wantStates)
			}

			if !reflect.DeepEqual(got.HighAvailability, tt.wantHA) {
				t.Errorf("HighAvailability =\n  %v\nwant\n  %v", got.HighAvailability, tt.wantHA)
			}

			if !reflect.DeepEqual(got.MajorVersions, tt.wantMajorVersions) {
				t.Errorf("MajorVersions =\n  %v\nwant\n  %v", got.MajorVersions, tt.wantMajorVersions)
			}
		})
	}
}
