package workload_test

import (
	"context"
	"testing"

	"github.com/nais/api/internal/workload"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
	"github.com/stretchr/testify/require"
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
	require.Equal(t, items[:2], filtered)

	empty := ""
	require.Equal(t, items, workload.SortFilter.Filter(context.Background(), items, &workload.TeamWorkloadsFilter{Name: &empty}))
}
