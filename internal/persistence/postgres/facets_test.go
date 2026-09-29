package postgres

import (
	"context"
	"reflect"
	"testing"

	"github.com/nais/api/internal/graph/model"
)

func TestComputeFacets(t *testing.T) {
	all := []*PostgresInstance{
		{Name: "first", EnvironmentName: "dev", State: PostgresInstanceStateAvailable},
		{Name: "second", EnvironmentName: "dev", State: PostgresInstanceStateProgressing},
		{Name: "third", EnvironmentName: "prod", State: PostgresInstanceStateDegraded},
	}
	tests := []struct {
		name             string
		filter           *PostgresInstanceFilter
		wantEnvironments []model.StringFacetItem
		wantStates       []PostgresInstanceStateFacetItem
	}{
		{"all", nil, []model.StringFacetItem{{Value: "dev", Count: 2}, {Value: "prod", Count: 1}}, []PostgresInstanceStateFacetItem{{State: PostgresInstanceStateAvailable, Count: 1}, {State: PostgresInstanceStateDegraded, Count: 1}, {State: PostgresInstanceStateProgressing, Count: 1}}},
		{"filter by environment", &PostgresInstanceFilter{Environments: []string{"dev"}}, []model.StringFacetItem{{Value: "dev", Count: 2}, {Value: "prod", Count: 0}}, []PostgresInstanceStateFacetItem{{State: PostgresInstanceStateAvailable, Count: 1}, {State: PostgresInstanceStateDegraded, Count: 0}, {State: PostgresInstanceStateProgressing, Count: 1}}},
		{"filter by state", &PostgresInstanceFilter{States: []PostgresInstanceState{PostgresInstanceStateAvailable}}, []model.StringFacetItem{{Value: "dev", Count: 1}, {Value: "prod", Count: 0}}, []PostgresInstanceStateFacetItem{{State: PostgresInstanceStateAvailable, Count: 1}, {State: PostgresInstanceStateDegraded, Count: 0}, {State: PostgresInstanceStateProgressing, Count: 0}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &PostgresInstanceFacets{AllInstances: all, Filter: tt.filter}
			if got := f.Environments(context.Background()); !reflect.DeepEqual(got, tt.wantEnvironments) {
				t.Errorf("environments=%v want %v", got, tt.wantEnvironments)
			}
			if got := f.States(context.Background()); !reflect.DeepEqual(got, tt.wantStates) {
				t.Errorf("states=%v want %v", got, tt.wantStates)
			}
		})
	}
}
