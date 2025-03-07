package job

import (
	"context"
	"slices"
	"strings"

	"github.com/nais/api/internal/graph/sortfilter"
)

var SortFilter = sortfilter.New[*Job, JobOrderField, *TeamJobsFilter]()

func init() {
	SortFilter.RegisterSort("NAME", func(ctx context.Context, a, b *Job) int {
		return strings.Compare(a.GetName(), b.GetName())
	}, "ENVIRONMENT")
	SortFilter.RegisterSort("ENVIRONMENT", func(ctx context.Context, a, b *Job) int {
		return strings.Compare(a.GetEnvironmentName(), b.GetEnvironmentName())
	}, "NAME")

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
