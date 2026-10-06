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

var vulnerabilityPriorityGrouping func(context.Context, []*Team, TeamVulnerabilityPriorityGroup) ([]*Team, error)

func RegisterVulnerabilityPriorityGrouping(fn func(context.Context, []*Team, TeamVulnerabilityPriorityGroup) ([]*Team, error)) {
	vulnerabilityPriorityGrouping = fn
}

func groupByVulnerabilityPriority(ctx context.Context, teams []*Team, priority TeamVulnerabilityPriorityGroup) ([]*Team, error) {
	if !priority.IsValid() {
		return nil, fmt.Errorf("invalid team vulnerability priority group: %s", priority)
	}
	if vulnerabilityPriorityGrouping == nil {
		return nil, fmt.Errorf("team vulnerability priority grouping is unavailable")
	}
	return vulnerabilityPriorityGrouping(ctx, teams, priority)
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
