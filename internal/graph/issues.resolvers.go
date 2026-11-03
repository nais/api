package graph

import (
	"context"

	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/issue"
	"github.com/nais/api/internal/persistence/opensearch"
	"github.com/nais/api/internal/persistence/postgres"
	"github.com/nais/api/internal/persistence/sqlinstance"
	"github.com/nais/api/internal/persistence/valkey"
	"github.com/nais/api/internal/team"
	"github.com/nais/api/internal/unleash"
	"github.com/nais/api/internal/workload"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
)

func (r *applicationRestartLoopIssueResolver) TeamEnvironment(ctx context.Context, obj *issue.ApplicationRestartLoopIssue) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
}

func (r *applicationRestartLoopIssueResolver) Workload(ctx context.Context, obj *issue.ApplicationRestartLoopIssue) (workload.Workload, error) {
	return getWorkloadByResourceType(ctx, obj.TeamSlug, obj.EnvironmentName, obj.ResourceName, obj.ResourceType)
}

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

func (r *externalIngressUrgentVulnerabilityIssueResolver) TeamEnvironment(ctx context.Context, obj *issue.ExternalIngressUrgentVulnerabilityIssue) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
}

func (r *externalIngressUrgentVulnerabilityIssueResolver) Workload(ctx context.Context, obj *issue.ExternalIngressUrgentVulnerabilityIssue) (workload.Workload, error) {
	return getWorkloadByResourceType(ctx, obj.TeamSlug, obj.EnvironmentName, obj.ResourceName, obj.ResourceType)
}

// Deprecated: superseded by WorkloadProblemIssue. Kept for GraphQL backwards compatibility.
func (r *failedSynchronizationIssueResolver) TeamEnvironment(ctx context.Context, obj *issue.FailedSynchronizationIssue) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
}

// Deprecated: superseded by WorkloadProblemIssue. Kept for GraphQL backwards compatibility.
func (r *failedSynchronizationIssueResolver) Workload(ctx context.Context, obj *issue.FailedSynchronizationIssue) (workload.Workload, error) {
	return getWorkloadByResourceType(ctx, obj.TeamSlug, obj.EnvironmentName, obj.ResourceName, obj.ResourceType)
}

// Deprecated: superseded by WorkloadProblemIssue. Kept for GraphQL backwards compatibility.
func (r *invalidSpecIssueResolver) TeamEnvironment(ctx context.Context, obj *issue.InvalidSpecIssue) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
}

// Deprecated: superseded by WorkloadProblemIssue. Kept for GraphQL backwards compatibility.
func (r *invalidSpecIssueResolver) Workload(ctx context.Context, obj *issue.InvalidSpecIssue) (workload.Workload, error) {
	return getWorkloadByResourceType(ctx, obj.TeamSlug, obj.EnvironmentName, obj.ResourceName, obj.ResourceType)
}

func (r *issueConnectionResolver) Facets(ctx context.Context, obj *issue.IssueConnection) (*issue.IssueFacets, error) {
	return issue.ComputeFacets(ctx, obj.GetTeamSlug(), obj.GetScope(), obj.GetFilter())
}

func (r *lastRunFailedIssueResolver) TeamEnvironment(ctx context.Context, obj *issue.LastRunFailedIssue) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
}

func (r *lastRunFailedIssueResolver) Job(ctx context.Context, obj *issue.LastRunFailedIssue) (*job.Job, error) {
	return job.Get(ctx, obj.TeamSlug, obj.EnvironmentName, obj.ResourceName)
}

func (r *missingSbomIssueResolver) TeamEnvironment(ctx context.Context, obj *issue.MissingSbomIssue) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
}

func (r *missingSbomIssueResolver) Workload(ctx context.Context, obj *issue.MissingSbomIssue) (workload.Workload, error) {
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

func (r *postgresBranchIssueResolver) TeamEnvironment(ctx context.Context, obj *issue.PostgresBranchIssue) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
}

func (r *postgresBranchIssueResolver) Branch(ctx context.Context, obj *issue.PostgresBranchIssue) (*postgres.PostgresBranch, error) {
	return postgres.GetPostgresBranchByObjectName(ctx, obj.TeamSlug, obj.EnvironmentName, obj.ResourceName)
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

func (r *teamResolver) Issues(ctx context.Context, obj *team.Team, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *issue.IssueOrder, filter *issue.IssueFilter) (*issue.IssueConnection, error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return issue.ListIssues(ctx, obj.Slug, page, orderBy, nil, filter)
}

func (r *unleashReleaseChannelIssueResolver) TeamEnvironment(ctx context.Context, obj *issue.UnleashReleaseChannelIssue) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
}

func (r *unleashReleaseChannelIssueResolver) Unleash(ctx context.Context, obj *issue.UnleashReleaseChannelIssue) (*unleash.UnleashInstance, error) {
	return unleash.ForTeam(ctx, obj.TeamSlug)
}

func (r *valkeyIssueResolver) TeamEnvironment(ctx context.Context, obj *issue.ValkeyIssue) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
}

func (r *valkeyIssueResolver) Valkey(ctx context.Context, obj *issue.ValkeyIssue) (*valkey.Valkey, error) {
	return valkey.Get(ctx, obj.TeamSlug, obj.EnvironmentName, obj.ResourceName)
}

func (r *vulnerableImageIssueResolver) TeamEnvironment(ctx context.Context, obj *issue.VulnerableImageIssue) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
}

func (r *vulnerableImageIssueResolver) Workload(ctx context.Context, obj *issue.VulnerableImageIssue) (workload.Workload, error) {
	return getWorkloadByResourceType(ctx, obj.TeamSlug, obj.EnvironmentName, obj.ResourceName, obj.ResourceType)
}

func (r *workloadProblemIssueResolver) TeamEnvironment(ctx context.Context, obj *issue.WorkloadProblemIssue) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
}

func (r *workloadProblemIssueResolver) Workload(ctx context.Context, obj *issue.WorkloadProblemIssue) (workload.Workload, error) {
	return getWorkloadByResourceType(ctx, obj.TeamSlug, obj.EnvironmentName, obj.ResourceName, obj.ResourceType)
}

func (r *Resolver) ApplicationRestartLoopIssue() gengql.ApplicationRestartLoopIssueResolver {
	return &applicationRestartLoopIssueResolver{r.resolver}
}

func (r *Resolver) DeprecatedIngressIssue() gengql.DeprecatedIngressIssueResolver {
	return &deprecatedIngressIssueResolver{r.resolver}
}

func (r *Resolver) DeprecatedRegistryIssue() gengql.DeprecatedRegistryIssueResolver {
	return &deprecatedRegistryIssueResolver{r.resolver}
}

func (r *Resolver) ExternalIngressUrgentVulnerabilityIssue() gengql.ExternalIngressUrgentVulnerabilityIssueResolver {
	return &externalIngressUrgentVulnerabilityIssueResolver{r.resolver}
}

func (r *Resolver) FailedSynchronizationIssue() gengql.FailedSynchronizationIssueResolver {
	return &failedSynchronizationIssueResolver{r.resolver}
}

func (r *Resolver) InvalidSpecIssue() gengql.InvalidSpecIssueResolver {
	return &invalidSpecIssueResolver{r.resolver}
}

func (r *Resolver) IssueConnection() gengql.IssueConnectionResolver {
	return &issueConnectionResolver{r.resolver}
}

func (r *Resolver) LastRunFailedIssue() gengql.LastRunFailedIssueResolver {
	return &lastRunFailedIssueResolver{r.resolver}
}

func (r *Resolver) MissingSbomIssue() gengql.MissingSbomIssueResolver {
	return &missingSbomIssueResolver{r.resolver}
}

func (r *Resolver) NoRunningInstancesIssue() gengql.NoRunningInstancesIssueResolver {
	return &noRunningInstancesIssueResolver{r.resolver}
}

func (r *Resolver) OpenSearchIssue() gengql.OpenSearchIssueResolver {
	return &openSearchIssueResolver{r.resolver}
}

func (r *Resolver) PostgresBranchIssue() gengql.PostgresBranchIssueResolver {
	return &postgresBranchIssueResolver{r.resolver}
}

func (r *Resolver) SqlInstanceStateIssue() gengql.SqlInstanceStateIssueResolver {
	return &sqlInstanceStateIssueResolver{r.resolver}
}

func (r *Resolver) SqlInstanceVersionIssue() gengql.SqlInstanceVersionIssueResolver {
	return &sqlInstanceVersionIssueResolver{r.resolver}
}

func (r *Resolver) UnleashReleaseChannelIssue() gengql.UnleashReleaseChannelIssueResolver {
	return &unleashReleaseChannelIssueResolver{r.resolver}
}

func (r *Resolver) ValkeyIssue() gengql.ValkeyIssueResolver { return &valkeyIssueResolver{r.resolver} }

func (r *Resolver) VulnerableImageIssue() gengql.VulnerableImageIssueResolver {
	return &vulnerableImageIssueResolver{r.resolver}
}

func (r *Resolver) WorkloadProblemIssue() gengql.WorkloadProblemIssueResolver {
	return &workloadProblemIssueResolver{r.resolver}
}

type (
	applicationRestartLoopIssueResolver             struct{ *resolver }
	deprecatedIngressIssueResolver                  struct{ *resolver }
	deprecatedRegistryIssueResolver                 struct{ *resolver }
	externalIngressUrgentVulnerabilityIssueResolver struct{ *resolver }
	failedSynchronizationIssueResolver              struct{ *resolver }
	invalidSpecIssueResolver                        struct{ *resolver }
	issueConnectionResolver                         struct{ *resolver }
	lastRunFailedIssueResolver                      struct{ *resolver }
	missingSbomIssueResolver                        struct{ *resolver }
	noRunningInstancesIssueResolver                 struct{ *resolver }
	openSearchIssueResolver                         struct{ *resolver }
	postgresBranchIssueResolver                     struct{ *resolver }
	sqlInstanceStateIssueResolver                   struct{ *resolver }
	sqlInstanceVersionIssueResolver                 struct{ *resolver }
	unleashReleaseChannelIssueResolver              struct{ *resolver }
	valkeyIssueResolver                             struct{ *resolver }
	vulnerableImageIssueResolver                    struct{ *resolver }
	workloadProblemIssueResolver                    struct{ *resolver }
)
