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
		return status(ctx, a.GetName(), ResourceTypeApplication, a.GetTeamSlug(), a.GetEnvironmentName())
	}, "NAME", "ENVIRONMENT")

	job.SortFilter.RegisterConcurrentSort("ISSUES", func(ctx context.Context, a *job.Job) int {
		return status(ctx, a.GetName(), ResourceTypeJob, a.GetTeamSlug(), a.GetEnvironmentName())
	}, "NAME", "ENVIRONMENT")

	workload.SortFilter.RegisterConcurrentSort("ISSUES", func(ctx context.Context, a workload.Workload) int {
		rtype := ResourceTypeApplication
		if a.GetType() == workload.TypeJob {
			rtype = ResourceTypeJob
		}
		return status(ctx, a.GetName(), rtype, a.GetTeamSlug(), a.GetEnvironmentName())
	}, "NAME", "ENVIRONMENT")
}

func status(ctx context.Context, name string, rtype ResourceType, team slug.Slug, env string) int {
	p := issuesql.GetSeverityScoreForWorkloadParams{
		ResourceName: name,
		ResourceType: rtype.String(),
		Env:          env,
		Team:         team.String(),
	}
	issuesScore, err := db(ctx).GetSeverityScoreForWorkload(ctx, p)
	if err != nil {
		return -1
	}
	return int(issuesScore)

	// params := issuesql.ListIssuesParams{
	// 	Team:         team.String(),
	// 	Env:          []string{env},
	// 	ResourceType: &typeptr,
	// 	ResourceName: &name,
	// 	Limit:        100,
	// }

	// issues, err := db(ctx).ListIssues(ctx, params)
	// if err != nil {
	// 	return -1
	// }
	// if len(issues) == 0 {
	// 	return -1
	// }
	// ret := 0
	// for _, i := range issues {
	// 	if i.Severity == issuesql.SeverityLevelCRITICAL {
	// 		return 2
	// 	}
	// 	if i.Severity == issuesql.SeverityLevelWARNING {
	// 		ret = 1
	// 	}
	// }
	// return ret
}
