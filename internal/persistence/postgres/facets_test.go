package postgres

import (
	"context"
	"reflect"
	"testing"

	"github.com/nais/api/internal/graph/model"
)

func TestComputeFacets(t *testing.T) {
	all := []*PostgresBranch{
		{Name: "first", EnvironmentName: "dev", State: PostgresBranchStateAvailable},
		{Name: "second", EnvironmentName: "dev", State: PostgresBranchStateProgressing},
		{Name: "third", EnvironmentName: "prod", State: PostgresBranchStateDegraded},
	}
	tests := []struct {
		name             string
		filter           *PostgresBranchFilter
		wantEnvironments []model.StringFacetItem
		wantStates       []PostgresBranchStateFacetItem
	}{
		{"all", nil, []model.StringFacetItem{{Value: "dev", Count: 2}, {Value: "prod", Count: 1}}, []PostgresBranchStateFacetItem{{State: PostgresBranchStateAvailable, Count: 1}, {State: PostgresBranchStateDegraded, Count: 1}, {State: PostgresBranchStateProgressing, Count: 1}}},
		{"filter by environment", &PostgresBranchFilter{Environments: []string{"dev"}}, []model.StringFacetItem{{Value: "dev", Count: 2}, {Value: "prod", Count: 0}}, []PostgresBranchStateFacetItem{{State: PostgresBranchStateAvailable, Count: 1}, {State: PostgresBranchStateDegraded, Count: 0}, {State: PostgresBranchStateProgressing, Count: 1}}},
		{"filter by state", &PostgresBranchFilter{States: []PostgresBranchState{PostgresBranchStateAvailable}}, []model.StringFacetItem{{Value: "dev", Count: 1}, {Value: "prod", Count: 0}}, []PostgresBranchStateFacetItem{{State: PostgresBranchStateAvailable, Count: 1}, {State: PostgresBranchStateDegraded, Count: 0}, {State: PostgresBranchStateProgressing, Count: 0}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &PostgresBranchFacets{AllInstances: all, Filter: tt.filter}
			if got := f.Environments(context.Background()); !reflect.DeepEqual(got, tt.wantEnvironments) {
				t.Errorf("environments=%v want %v", got, tt.wantEnvironments)
			}
			if got := f.States(context.Background()); !reflect.DeepEqual(got, tt.wantStates) {
				t.Errorf("states=%v want %v", got, tt.wantStates)
			}
		})
	}
}
