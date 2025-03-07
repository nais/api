package workload

import (
	"context"
	"slices"
	"strings"

	"github.com/nais/api/internal/graph/sortfilter"
)

var (
	SortFilter            = sortfilter.New[Workload, WorkloadOrderField, *TeamWorkloadsFilter]()
	SortFilterEnvironment = sortfilter.New[Workload, EnvironmentWorkloadOrderField, struct{}]()
)

func init() {
	SortFilter.RegisterSort("NAME", func(ctx context.Context, a, b Workload) int {
		return strings.Compare(a.GetName(), b.GetName())
	}, "ENVIRONMENT")
	SortFilter.RegisterSort("ENVIRONMENT", func(ctx context.Context, a, b Workload) int {
		return strings.Compare(a.GetEnvironmentName(), b.GetEnvironmentName())
	}, "NAME")

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
	}, "TEAM_SLUG")
	SortFilterEnvironment.RegisterSort("TEAM_SLUG", func(ctx context.Context, a, b Workload) int {
		return strings.Compare(a.GetTeamSlug().String(), b.GetTeamSlug().String())
	}, "NAME")
}
