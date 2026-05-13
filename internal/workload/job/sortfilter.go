package job

import (
	"context"
	"math"
	"strings"
	"time"

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
	SortFilter.RegisterSort("NEXT_RUN", func(ctx context.Context, a, b *Job) int {
		aNext := nextRunUnix(a)
		bNext := nextRunUnix(b)
		switch {
		case aNext < bNext:
			return -1
		case aNext > bNext:
			return 1
		default:
			return 0
		}
	}, "NAME", "ENVIRONMENT")

	SortFilter.RegisterFilter(matchesFilter)
}

func nextRunUnix(j *Job) int64 {
	s := j.Schedule()
	if s == nil || s.NextRun.IsZero() {
		return math.MaxInt64
	}
	return s.NextRun.Round(time.Second).Unix()
}
