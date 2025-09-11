package graph

import (
	"context"

	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/issue"
	"github.com/nais/api/internal/persistence/opensearch"
	"github.com/nais/api/internal/persistence/sqlinstance"
	"github.com/nais/api/internal/persistence/valkey"
	"github.com/nais/api/internal/team"
	"github.com/nais/api/internal/workload"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
)

func (r *deprecatedIngressIssueResolver) TeamEnvironment(ctx context.Context, obj *issue.DeprecatedIngressIssue) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
}

func (r *deprecatedIngressIssueResolver) Application(ctx context.Context, obj *issue.DeprecatedIngressIssue) (*application.Application, error) {
	return application.Get(ctx, obj.TeamSlug, obj.EnvironmentName, obj.ResourceName)
}

func (r *deprecatedRegistryIssueResolver) TeamEnvironment(ctx context.Context, obj *issue.DeprecatedRegistryIssue) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
}

func (r *deprecatedRegistryIssueResolver) Workload(ctx context.Context, obj *issue.DeprecatedRegistryIssue) (workload.Workload, error) {
	return getWorkloadByResourceType(ctx, obj.TeamSlug, obj.EnvironmentName, obj.ResourceName, obj.ResourceType)
}

func (r *failedGenerateIssueResolver) TeamEnvironment(ctx context.Context, obj *issue.FailedGenerateIssue) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
}

func (r *failedGenerateIssueResolver) Workload(ctx context.Context, obj *issue.FailedGenerateIssue) (workload.Workload, error) {
	return getWorkloadByResourceType(ctx, obj.TeamSlug, obj.EnvironmentName, obj.ResourceName, obj.ResourceType)
}

func (r *failedJobRunsIssueResolver) TeamEnvironment(ctx context.Context, obj *issue.FailedJobRunsIssue) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
}

func (r *failedJobRunsIssueResolver) Job(ctx context.Context, obj *issue.FailedJobRunsIssue) (*job.Job, error) {
	return job.Get(ctx, obj.TeamSlug, obj.EnvironmentName, obj.ResourceName)
}

func (r *failedSynchronizationIssueResolver) TeamEnvironment(ctx context.Context, obj *issue.FailedSynchronizationIssue) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
}

func (r *failedSynchronizationIssueResolver) Workload(ctx context.Context, obj *issue.FailedSynchronizationIssue) (workload.Workload, error) {
	return getWorkloadByResourceType(ctx, obj.TeamSlug, obj.EnvironmentName, obj.ResourceName, obj.ResourceType)
}

func (r *noRunningInstancesIssueResolver) TeamEnvironment(ctx context.Context, obj *issue.NoRunningInstancesIssue) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
}

func (r *noRunningInstancesIssueResolver) Workload(ctx context.Context, obj *issue.NoRunningInstancesIssue) (workload.Workload, error) {
	return getWorkloadByResourceType(ctx, obj.TeamSlug, obj.EnvironmentName, obj.ResourceName, obj.ResourceType)
}

func (r *openSearchIssueResolver) TeamEnvironment(ctx context.Context, obj *issue.OpenSearchIssue) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
}

func (r *openSearchIssueResolver) OpenSearch(ctx context.Context, obj *issue.OpenSearchIssue) (*opensearch.OpenSearch, error) {
	return opensearch.Get(ctx, obj.TeamSlug, obj.EnvironmentName, obj.ResourceName)
}

func (r *sqlInstanceStateIssueResolver) TeamEnvironment(ctx context.Context, obj *issue.SqlInstanceStateIssue) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
}

func (r *sqlInstanceStateIssueResolver) SQLInstance(ctx context.Context, obj *issue.SqlInstanceStateIssue) (*sqlinstance.SQLInstance, error) {
	return sqlinstance.Get(ctx, obj.TeamSlug, obj.EnvironmentName, obj.ResourceName)
}

func (r *sqlInstanceVersionIssueResolver) TeamEnvironment(ctx context.Context, obj *issue.SqlInstanceVersionIssue) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
}

func (r *sqlInstanceVersionIssueResolver) SQLInstance(ctx context.Context, obj *issue.SqlInstanceVersionIssue) (*sqlinstance.SQLInstance, error) {
	return sqlinstance.Get(ctx, obj.TeamSlug, obj.EnvironmentName, obj.ResourceName)
}

func (r *teamResolver) Issues(ctx context.Context, obj *team.Team, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *issue.IssueOrder, filter *issue.IssueFilter) (*pagination.Connection[issue.Issue], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return issue.ListIssues(ctx, obj.Slug, page, orderBy, filter)
}

func (r *valkeyIssueResolver) TeamEnvironment(ctx context.Context, obj *issue.ValkeyIssue) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
}

func (r *valkeyIssueResolver) Valkey(ctx context.Context, obj *issue.ValkeyIssue) (*valkey.Valkey, error) {
	return valkey.Get(ctx, obj.TeamSlug, obj.EnvironmentName, obj.ResourceName)
}

func (r *Resolver) DeprecatedIngressIssue() gengql.DeprecatedIngressIssueResolver {
	return &deprecatedIngressIssueResolver{r}
}

func (r *Resolver) DeprecatedRegistryIssue() gengql.DeprecatedRegistryIssueResolver {
	return &deprecatedRegistryIssueResolver{r}
}

func (r *Resolver) FailedGenerateIssue() gengql.FailedGenerateIssueResolver {
	return &failedGenerateIssueResolver{r}
}

func (r *Resolver) FailedJobRunsIssue() gengql.FailedJobRunsIssueResolver {
	return &failedJobRunsIssueResolver{r}
}

func (r *Resolver) FailedSynchronizationIssue() gengql.FailedSynchronizationIssueResolver {
	return &failedSynchronizationIssueResolver{r}
}

func (r *Resolver) NoRunningInstancesIssue() gengql.NoRunningInstancesIssueResolver {
	return &noRunningInstancesIssueResolver{r}
}

func (r *Resolver) OpenSearchIssue() gengql.OpenSearchIssueResolver {
	return &openSearchIssueResolver{r}
}

func (r *Resolver) SqlInstanceStateIssue() gengql.SqlInstanceStateIssueResolver {
	return &sqlInstanceStateIssueResolver{r}
}

func (r *Resolver) SqlInstanceVersionIssue() gengql.SqlInstanceVersionIssueResolver {
	return &sqlInstanceVersionIssueResolver{r}
}

func (r *Resolver) ValkeyIssue() gengql.ValkeyIssueResolver { return &valkeyIssueResolver{r} }

type (
	deprecatedIngressIssueResolver     struct{ *Resolver }
	deprecatedRegistryIssueResolver    struct{ *Resolver }
	failedGenerateIssueResolver        struct{ *Resolver }
	failedJobRunsIssueResolver         struct{ *Resolver }
	failedSynchronizationIssueResolver struct{ *Resolver }
	noRunningInstancesIssueResolver    struct{ *Resolver }
	openSearchIssueResolver            struct{ *Resolver }
	sqlInstanceStateIssueResolver      struct{ *Resolver }
	sqlInstanceVersionIssueResolver    struct{ *Resolver }
	valkeyIssueResolver                struct{ *Resolver }
)
