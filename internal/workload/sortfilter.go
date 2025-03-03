package workload

import (
	"context"
	"slices"
	"strings"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/sortfilter"
)

var (
	SortFilter            = sortfilter.New[Workload, WorkloadOrderField, *TeamWorkloadsFilter]("NAME", model.OrderDirectionAsc)
	SortFilterEnvironment = sortfilter.New[Workload, EnvironmentWorkloadOrderField, *struct{}]("NAME", model.OrderDirectionAsc)
)

func init() {
	SortFilter.RegisterSort("NAME", func(ctx context.Context, a, b Workload) int {
		return strings.Compare(a.GetName(), b.GetName())
	})
	SortFilter.RegisterSort("ENVIRONMENT", func(ctx context.Context, a, b Workload) int {
		return strings.Compare(a.GetEnvironmentName(), b.GetEnvironmentName())
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

	SortFilterEnvironment.RegisterSort("NAME", func(ctx context.Context, a, b Workload) int {
		return strings.Compare(a.GetName(), b.GetName())
	})

	SortFilterEnvironment.RegisterSort("TEAM_SLUG", func(ctx context.Context, a, b Workload) int {
		return strings.Compare(a.GetTeamSlug().String(), a.GetTeamSlug().String())
	})
}
