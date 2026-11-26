package graph

import (
	"context"

	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/team"
	"github.com/nais/api/internal/v0s"
	"github.com/nais/api/internal/workload"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
)

func (r *applicationResolver) VulnerabilityFindings(ctx context.Context, obj *application.Application, filter *v0s.WorkloadVulnerabilityFindingsFilter, first *int, after *pagination.Cursor, orderBy *v0s.VulnerabilityFindingOrder) (*v0s.VulnerabilityFindingConnection, error) {
	return v0s.FindingsForWorkloads(ctx, v0s.WorkloadScope(obj), []workload.Workload{obj}, (*v0s.VulnerabilityFindingFilter)(filter), first, after, orderBy)
}

func (r *applicationResolver) VulnerabilityCoverage(ctx context.Context, obj *application.Application) (*v0s.VulnerabilityCoverage, error) {
	return v0s.WorkloadCoverage(ctx, obj)
}

func (r *applicationResolver) VulnerabilityOverview(ctx context.Context, obj *application.Application, filter *v0s.WorkloadVulnerabilityOverviewFilter) (*v0s.VulnerabilityWorkloadOverview, error) {
	return v0s.WorkloadOverview(ctx, obj, (*v0s.VulnerabilityFindingFilter)(filter))
}

func (r *jobResolver) VulnerabilityFindings(ctx context.Context, obj *job.Job, filter *v0s.WorkloadVulnerabilityFindingsFilter, first *int, after *pagination.Cursor, orderBy *v0s.VulnerabilityFindingOrder) (*v0s.VulnerabilityFindingConnection, error) {
	return v0s.FindingsForWorkloads(ctx, v0s.WorkloadScope(obj), []workload.Workload{obj}, (*v0s.VulnerabilityFindingFilter)(filter), first, after, orderBy)
}

func (r *jobResolver) VulnerabilityCoverage(ctx context.Context, obj *job.Job) (*v0s.VulnerabilityCoverage, error) {
	return v0s.WorkloadCoverage(ctx, obj)
}

func (r *jobResolver) VulnerabilityOverview(ctx context.Context, obj *job.Job, filter *v0s.WorkloadVulnerabilityOverviewFilter) (*v0s.VulnerabilityWorkloadOverview, error) {
	return v0s.WorkloadOverview(ctx, obj, (*v0s.VulnerabilityFindingFilter)(filter))
}

func (r *mutationResolver) SetVulnerabilitySuppressions(ctx context.Context, input v0s.SetVulnerabilitySuppressionsInput) (*v0s.SetVulnerabilitySuppressionsPayload, error) {
	return v0s.SetSuppressions(ctx, input)
}

func (r *mutationResolver) DeleteVulnerabilitySuppressions(ctx context.Context, input v0s.DeleteVulnerabilitySuppressionsInput) (*v0s.DeleteVulnerabilitySuppressionsPayload, error) {
	return v0s.DeleteSuppressions(ctx, input)
}

func (r *queryResolver) VulnerabilityFindings(ctx context.Context, filter *v0s.VulnerabilityFindingsFilter, first *int, after *pagination.Cursor, orderBy *v0s.VulnerabilityFindingOrder) (*v0s.VulnerabilityFindingConnection, error) {
	var metadata *v0s.VulnerabilityWorkloadFilter
	var findings *v0s.VulnerabilityFindingFilter
	if filter != nil {
		metadata = &v0s.VulnerabilityWorkloadFilter{TeamScope: filter.TeamScope, Environments: filter.Environments, Labels: filter.Labels}
		findings = &filter.VulnerabilityFindingFilter
	}
	selected, err := selectVulnerabilityWorkloads(ctx, metadata)
	if err != nil {
		return nil, err
	}
	return v0s.FindingsForSelection(ctx, v0s.Scope{}, selected, metadata, findings, first, after, orderBy)
}

func (r *queryResolver) Vulnerability(ctx context.Context, identifier string) (*v0s.Vulnerability, error) {
	return v0s.VulnerabilityByIdentifier(ctx, identifier)
}

func (r *queryResolver) VulnerabilitySuppressions(ctx context.Context, filter *v0s.VulnerabilitySuppressionFilter, first *int, after *pagination.Cursor) (*pagination.Connection[*v0s.VulnerabilitySuppression], error) {
	return v0s.Suppressions(ctx, v0s.Scope{}, filter, first, after)
}

func (r *teamResolver) VulnerabilityFindings(ctx context.Context, obj *team.Team, filter *v0s.TeamVulnerabilityFindingsFilter, first *int, after *pagination.Cursor, orderBy *v0s.VulnerabilityFindingOrder) (*v0s.VulnerabilityFindingConnection, error) {
	scope := v0s.Scope{Team: obj.Slug}
	var workloadFilter *workload.TeamWorkloadsFilter
	var metadata *v0s.VulnerabilityWorkloadFilter
	var findings *v0s.VulnerabilityFindingFilter
	if filter != nil {
		workloadFilter = &workload.TeamWorkloadsFilter{Environments: filter.Environments, Labels: filter.Labels}
		metadata = &v0s.VulnerabilityWorkloadFilter{Environments: filter.Environments, Labels: filter.Labels}
		findings = &filter.VulnerabilityFindingFilter
	}
	return v0s.FindingsForSelection(ctx, scope, selectTeamWorkloads(ctx, obj.Slug, workloadFilter), metadata, findings, first, after, orderBy)
}

func (r *teamResolver) VulnerabilitySuppressions(ctx context.Context, obj *team.Team, filter *v0s.TeamVulnerabilitySuppressionsFilter, first *int, after *pagination.Cursor) (*pagination.Connection[*v0s.VulnerabilitySuppression], error) {
	return v0s.Suppressions(ctx, v0s.Scope{Team: obj.Slug}, (*v0s.VulnerabilitySuppressionFilter)(filter), first, after)
}

func (r *teamEnvironmentResolver) VulnerabilityFindings(ctx context.Context, obj *team.TeamEnvironment, filter *v0s.TeamEnvironmentVulnerabilityFindingsFilter, first *int, after *pagination.Cursor, orderBy *v0s.VulnerabilityFindingOrder) (*v0s.VulnerabilityFindingConnection, error) {
	selected := selectTeamWorkloads(ctx, obj.TeamSlug, &workload.TeamWorkloadsFilter{Environments: []string{obj.EnvironmentName}})
	return v0s.FindingsForWorkloads(ctx, v0s.Scope{Team: obj.TeamSlug, Environment: obj.EnvironmentName}, selected, (*v0s.VulnerabilityFindingFilter)(filter), first, after, orderBy)
}

func (r *teamEnvironmentResolver) VulnerabilitySuppressions(ctx context.Context, obj *team.TeamEnvironment, filter *v0s.TeamEnvironmentVulnerabilitySuppressionsFilter, first *int, after *pagination.Cursor) (*pagination.Connection[*v0s.VulnerabilitySuppression], error) {
	return v0s.Suppressions(ctx, v0s.Scope{Team: obj.TeamSlug, Environment: obj.EnvironmentName}, (*v0s.VulnerabilitySuppressionFilter)(filter), first, after)
}

func (r *vulnerabilityFindingResolver) Workload(ctx context.Context, obj *v0s.VulnerabilityFinding) (workload.Workload, error) {
	ref := obj.Workload
	return tryWorkload(ctx, ref.Team, ref.Environment, ref.Name)
}

func (r *vulnerabilityFindingResolver) Vulnerability(ctx context.Context, obj *v0s.VulnerabilityFinding) (*v0s.Vulnerability, error) {
	return v0s.VulnerabilityByIdentifier(ctx, obj.VulnerabilityIdentifier)
}

func (r *vulnerabilityOverviewResolver) Vulnerability(ctx context.Context, obj *v0s.VulnerabilityOverview) (*v0s.Vulnerability, error) {
	return v0s.VulnerabilityByIdentifier(ctx, obj.VulnerabilityIdentifier)
}

func (r *vulnerabilitySuppressionResolver) Workload(ctx context.Context, obj *v0s.VulnerabilitySuppression) (workload.Workload, error) {
	return tryWorkload(ctx, obj.Team, obj.Environment, obj.Workload)
}

func (r *vulnerabilitySuppressionResolver) Vulnerability(ctx context.Context, obj *v0s.VulnerabilitySuppression) (*v0s.Vulnerability, error) {
	return v0s.VulnerabilityByIdentifier(ctx, obj.VulnerabilityIdentifier)
}

func (r *Resolver) VulnerabilityFinding() gengql.VulnerabilityFindingResolver {
	return &vulnerabilityFindingResolver{r}
}

func (r *Resolver) VulnerabilityOverview() gengql.VulnerabilityOverviewResolver {
	return &vulnerabilityOverviewResolver{r}
}

func (r *Resolver) VulnerabilitySuppression() gengql.VulnerabilitySuppressionResolver {
	return &vulnerabilitySuppressionResolver{r}
}

type (
	vulnerabilityFindingResolver     struct{ *Resolver }
	vulnerabilityOverviewResolver    struct{ *Resolver }
	vulnerabilitySuppressionResolver struct{ *Resolver }
)
