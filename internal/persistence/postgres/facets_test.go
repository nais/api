package postgres

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nais/api/internal/graph/model"
)

func TestPostgresFacets(t *testing.T) {
	all := []*Postgres{
		{Name: "first", EnvironmentName: "dev", Labels: []*model.ResourceLabel{{Key: "purpose", Value: "test"}}},
		{Name: "second", EnvironmentName: "prod", Labels: []*model.ResourceLabel{{Key: "purpose", Value: "live"}}},
	}
	filter := &TeamPostgresFilter{Environments: []string{"dev"}}
	var filtered []*Postgres
	for _, pg := range all {
		if filter.Matches(pg) {
			filtered = append(filtered, pg)
		}
	}
	if len(filtered) != 1 || filtered[0] != all[0] {
		t.Fatalf("filtered = %+v", filtered)
	}
	facets := (&PostgresFacets{AllInstances: all, FilteredInstances: filtered}).Labels(t.Context())
	want := model.ComputeLabelsFacet(all, filtered, func(pg *Postgres) []*model.ResourceLabel { return pg.Labels })
	if diff := cmp.Diff(want, facets); diff != "" {
		t.Errorf("facets (-want +got):\n%s", diff)
	}
}
