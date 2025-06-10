package team

import (
	"context"
	"strings"

	"github.com/nais/api/internal/graph/sortfilter"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
)

var SortFilter = sortfilter.New[*Team, TeamOrderField, *TeamFilter]()

func init() {
	SortFilter.RegisterSort("_SLUG", func(ctx context.Context, a, b *Team) int {
		return strings.Compare(a.Slug.String(), b.Slug.String())
	})

	SortFilter.RegisterFilter(func(ctx context.Context, v *Team, filter *TeamFilter) bool {
		if filter.HasWorkloads {
			apps := application.ListAllForTeam(ctx, v.Slug, nil, nil)
			if len(apps) > 0 {
				return true
			}
			jobs := job.ListAllForTeam(ctx, v.Slug, nil, nil)
			if len(jobs) > 0 {
				return true
			}
		}
		return false
	})

}
