package graph

import (
	"context"

	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/team"
	"github.com/nais/api/internal/v0s"
	"github.com/nais/api/internal/v0s/filter"
	"github.com/nais/api/internal/workload"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
)

func (r *applicationResolver) VulnerabilityFindings(ctx context.Context, obj *application.Application, filter *filter.VulnerabilityFindingFilter, first *int, after *pagination.Cursor, orderBy *v0s.VulnerabilityFindingOrder) (*v0s.VulnerabilityFindingConnection, error) {
	return v0s.Findings(ctx, v0s.WorkloadScope(obj), filter, first, after, orderBy)
}

func (r *applicationResolver) VulnerabilityCoverage(ctx context.Context, obj *application.Application) (*v0s.VulnerabilityCoverage, error) {
	return v0s.WorkloadCoverage(ctx, obj)
}

func (r *applicationResolver) VulnerabilityOverview(ctx context.Context, obj *application.Application, filter *filter.VulnerabilityFindingFilter) (*v0s.VulnerabilityWorkloadOverview, error) {
	return v0s.WorkloadOverview(ctx, obj, filter)
}

func (r *jobResolver) VulnerabilityFindings(ctx context.Context, obj *job.Job, filter *filter.VulnerabilityFindingFilter, first *int, after *pagination.Cursor, orderBy *v0s.VulnerabilityFindingOrder) (*v0s.VulnerabilityFindingConnection, error) {
	return v0s.Findings(ctx, v0s.WorkloadScope(obj), filter, first, after, orderBy)
}

func (r *jobResolver) VulnerabilityCoverage(ctx context.Context, obj *job.Job) (*v0s.VulnerabilityCoverage, error) {
	return v0s.WorkloadCoverage(ctx, obj)
}

func (r *jobResolver) VulnerabilityOverview(ctx context.Context, obj *job.Job, filter *filter.VulnerabilityFindingFilter) (*v0s.VulnerabilityWorkloadOverview, error) {
	return v0s.WorkloadOverview(ctx, obj, filter)
}

func (r *mutationResolver) SetVulnerabilitySuppressions(ctx context.Context, input v0s.SetVulnerabilitySuppressionsInput) (*v0s.SetVulnerabilitySuppressionsPayload, error) {
	return v0s.SetSuppressions(ctx, input)
}

func (r *mutationResolver) DeleteVulnerabilitySuppressions(ctx context.Context, input v0s.DeleteVulnerabilitySuppressionsInput) (*v0s.DeleteVulnerabilitySuppressionsPayload, error) {
	return v0s.DeleteSuppressions(ctx, input)
}

func (r *queryResolver) VulnerabilityFindings(ctx context.Context, filter *filter.VulnerabilityFindingFilter, first *int, after *pagination.Cursor, orderBy *v0s.VulnerabilityFindingOrder) (*v0s.VulnerabilityFindingConnection, error) {
	return v0s.Findings(ctx, v0s.Scope{}, filter, first, after, orderBy)
}

func (r *queryResolver) Vulnerability(ctx context.Context, identifier string) (*v0s.Vulnerability, error) {
	return v0s.VulnerabilityByIdentifier(ctx, identifier)
}

func (r *queryResolver) VulnerabilitySuppressions(ctx context.Context, filter *v0s.VulnerabilitySuppressionFilter, first *int, after *pagination.Cursor) (*pagination.Connection[*v0s.VulnerabilitySuppression], error) {
	return v0s.Suppressions(ctx, v0s.Scope{}, filter, first, after)
}

func (r *teamResolver) VulnerabilityFindings(ctx context.Context, obj *team.Team, filter *filter.VulnerabilityFindingFilter, first *int, after *pagination.Cursor, orderBy *v0s.VulnerabilityFindingOrder, workloadFilter *workload.TeamWorkloadsFilter) (*v0s.VulnerabilityFindingConnection, error) {
	scope := v0s.Scope{Team: obj.Slug}
	if workloadFilter != nil && workloadFilter.VulnerabilityFilter != nil {
		return nil, apierror.Errorf("Use filter to select vulnerability findings; workloadFilter only supports environments and labels")
	}
	if err := v0s.ValidateWorkloadFilter(workloadFilter); err != nil {
		return nil, err
	}
	return v0s.FindingsForWorkloads(ctx, scope, selectTeamWorkloads(ctx, obj.Slug, workloadFilter), filter, first, after, orderBy)
}

func (r *teamResolver) VulnerabilitySuppressions(ctx context.Context, obj *team.Team, filter *v0s.VulnerabilitySuppressionFilter, first *int, after *pagination.Cursor) (*pagination.Connection[*v0s.VulnerabilitySuppression], error) {
	return v0s.Suppressions(ctx, v0s.Scope{Team: obj.Slug}, filter, first, after)
}

func (r *teamEnvironmentResolver) VulnerabilityFindings(ctx context.Context, obj *team.TeamEnvironment, filter *filter.VulnerabilityFindingFilter, first *int, after *pagination.Cursor, orderBy *v0s.VulnerabilityFindingOrder) (*v0s.VulnerabilityFindingConnection, error) {
	return v0s.Findings(ctx, v0s.Scope{Team: obj.TeamSlug, Environment: obj.EnvironmentName}, filter, first, after, orderBy)
}

func (r *teamEnvironmentResolver) VulnerabilitySuppressions(ctx context.Context, obj *team.TeamEnvironment, filter *v0s.VulnerabilitySuppressionFilter, first *int, after *pagination.Cursor) (*pagination.Connection[*v0s.VulnerabilitySuppression], error) {
	return v0s.Suppressions(ctx, v0s.Scope{Team: obj.TeamSlug, Environment: obj.EnvironmentName}, filter, first, after)
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
