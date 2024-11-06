package job

import (
	"context"
	"strings"

	"github.com/nais/api/internal/graph/sortfilter"
)

var SortFilter = sortfilter.New[*Job, JobOrderField, *TeamJobsFilter](JobOrderFieldName)

func init() {
	SortFilter.RegisterOrderBy(JobOrderFieldName, func(ctx context.Context, a, b *Job) int {
		return strings.Compare(a.GetName(), b.GetName())
	})
	SortFilter.RegisterOrderBy(JobOrderFieldEnvironment, func(ctx context.Context, a, b *Job) int {
		return strings.Compare(a.GetEnvironmentName(), b.GetEnvironmentName())
	})
	SortFilter.RegisterFilter(func(ctx context.Context, v *Job, filter *TeamJobsFilter) bool {
		return strings.Contains(strings.ToLower(v.Name), strings.ToLower(filter.Name))
	})
}
