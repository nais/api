package issue

import (
	"context"

	"github.com/nais/api/internal/issue/issuesql"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/workload"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
)

func init() {
	application.SortFilter.RegisterConcurrentSort("ISSUES", func(ctx context.Context, a *application.Application) int {
		return score(ctx, a.GetName(), ResourceTypeApplication, a.GetTeamSlug(), a.GetEnvironmentName())
	}, "NAME", "ENVIRONMENT")

	job.SortFilter.RegisterConcurrentSort("ISSUES", func(ctx context.Context, a *job.Job) int {
		return score(ctx, a.GetName(), ResourceTypeJob, a.GetTeamSlug(), a.GetEnvironmentName())
	}, "NAME", "ENVIRONMENT")

	workload.SortFilter.RegisterConcurrentSort("ISSUES", func(ctx context.Context, a workload.Workload) int {
		rtype := ResourceTypeApplication
		if a.GetType() == workload.TypeJob {
			rtype = ResourceTypeJob
		}
		return score(ctx, a.GetName(), rtype, a.GetTeamSlug(), a.GetEnvironmentName())
	}, "NAME", "ENVIRONMENT")
}

func score(ctx context.Context, name string, rtype ResourceType, team slug.Slug, env string) int {
	issuesScore, err := db(ctx).GetSeverityScoreForWorkload(ctx,
		issuesql.GetSeverityScoreForWorkloadParams{
			ResourceName: name,
			ResourceType: rtype.String(),
			Env:          env,
			Team:         team.String(),
		})
	if err != nil {
		return -1
	}

	return int(issuesScore)
}
