package job

import (
	"context"
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
	SortFilter.RegisterSort("NEXT_RUN", func(ctx context.Context, a, b *Job) int {
		aNext := a.Schedule()
		bNext := b.Schedule()
		aHas := aNext != nil && aNext.NextRun != nil
		bHas := bNext != nil && bNext.NextRun != nil

		switch {
		case !aHas && !bHas:
			return 0
		case !aHas:
			return 1
		case !bHas:
			return -1
		}

		switch {
		case aNext.NextRun.Before(*bNext.NextRun):
			return -1
		case aNext.NextRun.After(*bNext.NextRun):
			return 1
		default:
			return 0
		}
	}, "NAME", "ENVIRONMENT")

	SortFilter.RegisterFilter(matchesFilter)
}
