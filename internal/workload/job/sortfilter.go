package job

import (
	"context"
	"slices"
	"strings"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/sortfilter"
	"k8s.io/utils/ptr"
)

var SortFilter = sortfilter.New[*Job, JobOrderField, *TeamJobsFilter]()

type SortFilterTieBreaker = sortfilter.TieBreaker[JobOrderField]

func init() {
	SortFilter.RegisterSort("NAME", func(ctx context.Context, a, b *Job) int {
		return strings.Compare(a.GetName(), b.GetName())
	}, SortFilterTieBreaker{
		Field:     "ENVIRONMENT",
		Direction: ptr.To(model.OrderDirectionAsc),
	})
	SortFilter.RegisterSort("ENVIRONMENT", func(ctx context.Context, a, b *Job) int {
		return strings.Compare(a.GetEnvironmentName(), b.GetEnvironmentName())
	}, SortFilterTieBreaker{
		Field:     "NAME",
		Direction: ptr.To(model.OrderDirectionAsc),
	})

	SortFilter.RegisterFilter(func(ctx context.Context, v *Job, filter *TeamJobsFilter) bool {
		if filter.Name != "" {
			if !strings.Contains(strings.ToLower(v.Name), strings.ToLower(filter.Name)) {
				return false
			}
		}

		if len(filter.Environments) > 0 {
			if !slices.Contains(filter.Environments, v.EnvironmentName) {
				return false
			}
		}

		return true
	})
}
