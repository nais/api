package workload

import (
	"context"
	"slices"
	"strings"

	"github.com/nais/api/internal/v1/graphv1/sortfilter"
)

var SortFilter = sortfilter.New[Workload, WorkloadOrderField, *TeamWorkloadsFilter](WorkloadOrderFieldName)

func init() {
	SortFilter.RegisterOrderBy(WorkloadOrderFieldName, func(ctx context.Context, a, b Workload) int {
		return strings.Compare(a.GetName(), b.GetName())
	})
	SortFilter.RegisterOrderBy(WorkloadOrderFieldEnvironment, func(ctx context.Context, a, b Workload) int {
		return strings.Compare(a.GetEnvironmentName(), b.GetEnvironmentName())
	})
	SortFilter.RegisterOrderBy(WorkloadOrderFieldDeploymentTime, func(ctx context.Context, a, b Workload) int {
		return int(a.GetRolloutCompleteTime() - b.GetRolloutCompleteTime())
	})
	SortFilter.RegisterFilter(func(ctx context.Context, v Workload, filter *TeamWorkloadsFilter) bool {
		if len(filter.Environments) == 0 {
			return true
		}

		if slices.Contains(filter.Environments, v.GetEnvironmentName()) {
			return true
		}
		return false
	})
}
