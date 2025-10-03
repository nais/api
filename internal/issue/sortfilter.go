package issue

import (
	"context"

	"github.com/nais/api/internal/issue/issuesql"
	"github.com/nais/api/internal/persistence/opensearch"
	"github.com/nais/api/internal/persistence/sqlinstance"
	"github.com/nais/api/internal/persistence/valkey"
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

	sqlinstance.SortFilterSQLInstance.RegisterConcurrentSort("ISSUES", func(ctx context.Context, a *sqlinstance.SQLInstance) int {
		return score(ctx, a.GetName(), ResourceTypeSQLInstance, a.TeamSlug, a.EnvironmentName)
	}, "NAME", "ENVIRONMENT")

	opensearch.SortFilterOpenSearch.RegisterConcurrentSort("ISSUES", func(ctx context.Context, a *opensearch.OpenSearch) int {
		return score(ctx, a.GetName(), ResourceTypeOpensearch, a.TeamSlug, a.EnvironmentName)
	}, "NAME", "ENVIRONMENT")

	valkey.SortFilterValkey.RegisterConcurrentSort("ISSUES", func(ctx context.Context, a *valkey.Valkey) int {
		return score(ctx, a.GetName(), ResourceTypeValkey, a.TeamSlug, a.EnvironmentName)
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
