package workload_test

import (
	"context"
	"slices"
	"testing"

	"github.com/nais/api/internal/workload"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
)

func TestTeamWorkloadNameFilterMatchesAcrossTypesAndEnvironments(t *testing.T) {
	items := []workload.Workload{
		&application.Application{Base: workload.Base{Name: "payments-api", EnvironmentName: "dev"}},
		&job.Job{Base: workload.Base{Name: "PAYMENTS-CLEANUP", EnvironmentName: "dev"}},
		&application.Application{Base: workload.Base{Name: "payments-api", EnvironmentName: "prod"}},
		&job.Job{Base: workload.Base{Name: "unrelated", EnvironmentName: "dev"}},
	}
	name := " Payments "
	filtered := workload.SortFilter.Filter(context.Background(), items, &workload.TeamWorkloadsFilter{
		Name: &name, Environments: []string{"dev"},
	})
	if len(filtered) != 2 || !slices.Contains(filtered, items[0]) || !slices.Contains(filtered, items[1]) {
		t.Errorf("filtered workloads = %v, want %v and %v", filtered, items[0], items[1])
	}

	empty := ""
	filtered = workload.SortFilter.Filter(context.Background(), items, &workload.TeamWorkloadsFilter{Name: &empty})
	if len(filtered) != len(items) {
		t.Fatalf("filtered workloads = %v, want all %d workloads", filtered, len(items))
	}
	for _, item := range items {
		if !slices.Contains(filtered, item) {
			t.Errorf("filtered workloads missing %v", item)
		}
	}
}
