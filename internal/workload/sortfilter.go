package workload

import (
	"context"
	"slices"
	"strings"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/sortfilter"
	"k8s.io/utils/ptr"
)

var (
	SortFilter            = sortfilter.New[Workload, WorkloadOrderField, *TeamWorkloadsFilter]()
	SortFilterEnvironment = sortfilter.New[Workload, EnvironmentWorkloadOrderField, struct{}]()
)

type (
	SortFilterTieBreaker            = sortfilter.TieBreaker[WorkloadOrderField]
	SortFilterEnvironmentTieBreaker = sortfilter.TieBreaker[EnvironmentWorkloadOrderField]
)

func init() {
	SortFilter.RegisterSort("NAME", func(ctx context.Context, a, b Workload) int {
		return strings.Compare(a.GetName(), b.GetName())
	}, SortFilterTieBreaker{
		Field:     "ENVIRONMENT",
		Direction: ptr.To(model.OrderDirectionAsc),
	})
	SortFilter.RegisterSort("ENVIRONMENT", func(ctx context.Context, a, b Workload) int {
		return strings.Compare(a.GetEnvironmentName(), b.GetEnvironmentName())
	}, SortFilterTieBreaker{
		Field:     "NAME",
		Direction: ptr.To(model.OrderDirectionAsc),
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

	SortFilterEnvironment.RegisterSort(
		"NAME",
		func(ctx context.Context, a, b Workload) int {
			return strings.Compare(a.GetName(), b.GetName())
		},
		SortFilterEnvironmentTieBreaker{
			Field:     "TEAM_SLUG",
			Direction: ptr.To(model.OrderDirectionAsc),
		},
	)
	SortFilterEnvironment.RegisterSort(
		"TEAM_SLUG",
		func(ctx context.Context, a, b Workload) int {
			return strings.Compare(a.GetTeamSlug().String(), b.GetTeamSlug().String())
		},
		SortFilterEnvironmentTieBreaker{
			Field:     "NAME",
			Direction: ptr.To(model.OrderDirectionAsc),
		},
	)
}
