package job

import (
	"context"
	"math"
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
	SortFilter.RegisterConcurrentSort("STATE", func(ctx context.Context, a *Job) int {
		s, err := GetState(ctx, a)
		if err != nil {
			return int(JobStateUnknown)
		}
		return int(s)
	}, "NAME", "ENVIRONMENT")
	SortFilter.RegisterConcurrentSort("NEXT_RUN", func(ctx context.Context, j *Job) int {
		return int(nextRunUnix(j))
	}, "NAME", "ENVIRONMENT")

	SortFilter.RegisterFilter(matchesFilter)
}

func nextRunUnix(j *Job) int64 {
	s := j.Schedule()
	if s == nil {
		return math.MaxInt64
	}
	return s.NextRun.Unix()
}
