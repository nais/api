package workload

import (
	"context"
	"slices"
	"strings"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/sortfilter"
)

var SortFilter = sortfilter.New[Workload, WorkloadOrderField, *TeamWorkloadsFilter]("NAME", model.OrderDirectionAsc)

func init() {
	SortFilter.RegisterOrderBy("NAME", func(ctx context.Context, a, b Workload) int {
		return strings.Compare(a.GetName(), b.GetName())
	})
	SortFilter.RegisterOrderBy("ENVIRONMENT", func(ctx context.Context, a, b Workload) int {
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
}
