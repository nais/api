package team

import (
	"context"
	"fmt"
	"strings"

	"github.com/nais/api/internal/graph/sortfilter"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
)

var SortFilter = sortfilter.New[*Team, TeamOrderField, *TeamFilter]()

var operationalPriorityGrouping func(context.Context, []*Team, TeamOperationalPriority) ([]*Team, error)

func RegisterOperationalPriorityGrouping(fn func(context.Context, []*Team, TeamOperationalPriority) ([]*Team, error)) {
	operationalPriorityGrouping = fn
}

func groupByOperationalPriority(ctx context.Context, teams []*Team, priority TeamOperationalPriority) ([]*Team, error) {
	if !priority.IsValid() {
		return nil, fmt.Errorf("invalid team operational priority: %s", priority)
	}
	if operationalPriorityGrouping == nil {
		return nil, fmt.Errorf("team operational priority grouping is unavailable")
	}
	return operationalPriorityGrouping(ctx, teams, priority)
}

func init() {
	SortFilter.RegisterSort("_SLUG", func(ctx context.Context, a, b *Team) int {
		return strings.Compare(a.Slug.String(), b.Slug.String())
	})

	SortFilter.RegisterFilter(func(ctx context.Context, v *Team, filter *TeamFilter) bool {
		if filter.HasWorkloads == nil {
			return true
		}
		apps := application.ListAllForTeam(ctx, v.Slug, nil, nil)
		if len(apps) > 0 {
			return *filter.HasWorkloads
		}
		jobs := job.ListAllForTeam(ctx, v.Slug, nil, nil)
		if len(jobs) > 0 {
			return *filter.HasWorkloads
		}
		return !*filter.HasWorkloads
	})
}
