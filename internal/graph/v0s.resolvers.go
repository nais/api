package graph

import (
	"context"

	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/v0s"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
)

func (r *applicationResolver) VulnerabilityFindings(ctx context.Context, obj *application.Application, filter *v0s.VulnerabilityFindingFilter, first *int, after *pagination.Cursor) (*pagination.Connection[*v0s.VulnerabilityFinding], error) {
	return v0s.Findings(ctx, v0s.WorkloadScope(obj), filter, first, after)
}

func (r *applicationResolver) VulnerabilityFindingSummary(ctx context.Context, obj *application.Application, filter *v0s.VulnerabilityFindingFilter) (*v0s.VulnerabilityFindingSummary, error) {
	return v0s.FindingSummary(ctx, v0s.WorkloadScope(obj), filter)
}

func (r *applicationResolver) VulnerabilityCoverage(ctx context.Context, obj *application.Application) (*v0s.VulnerabilityCoverage, error) {
	return v0s.WorkloadCoverage(ctx, obj)
}

func (r *jobResolver) VulnerabilityFindings(ctx context.Context, obj *job.Job, filter *v0s.VulnerabilityFindingFilter, first *int, after *pagination.Cursor) (*pagination.Connection[*v0s.VulnerabilityFinding], error) {
	return v0s.Findings(ctx, v0s.WorkloadScope(obj), filter, first, after)
}

func (r *jobResolver) VulnerabilityFindingSummary(ctx context.Context, obj *job.Job, filter *v0s.VulnerabilityFindingFilter) (*v0s.VulnerabilityFindingSummary, error) {
	return v0s.FindingSummary(ctx, v0s.WorkloadScope(obj), filter)
}

func (r *jobResolver) VulnerabilityCoverage(ctx context.Context, obj *job.Job) (*v0s.VulnerabilityCoverage, error) {
	return v0s.WorkloadCoverage(ctx, obj)
}

func (r *mutationResolver) SetVulnerabilitySuppressions(ctx context.Context, input v0s.SetVulnerabilitySuppressionsInput) (*v0s.SetVulnerabilitySuppressionsPayload, error) {
	return v0s.SetSuppressions(ctx, input)
}

func (r *mutationResolver) DeleteVulnerabilitySuppressions(ctx context.Context, input v0s.DeleteVulnerabilitySuppressionsInput) (*v0s.DeleteVulnerabilitySuppressionsPayload, error) {
	return v0s.DeleteSuppressions(ctx, input)
}

func (r *queryResolver) VulnerabilityFindings(ctx context.Context, filter *v0s.VulnerabilityFindingFilter, first *int, after *pagination.Cursor) (*pagination.Connection[*v0s.VulnerabilityFinding], error) {
	return v0s.Findings(ctx, nil, filter, first, after)
}

func (r *queryResolver) VulnerabilityFindingSummary(ctx context.Context, filter *v0s.VulnerabilityFindingFilter) (*v0s.VulnerabilityFindingSummary, error) {
	return v0s.FindingSummary(ctx, nil, filter)
}

func (r *queryResolver) VulnerabilityCoverage(ctx context.Context, scope *v0s.VulnerabilityScopeInput) (*v0s.VulnerabilityCoverageCounts, error) {
	return v0s.Coverage(ctx, scope)
}

func (r *queryResolver) Vulnerability(ctx context.Context, identifier string) (*v0s.Vulnerability, error) {
	return v0s.VulnerabilityByIdentifier(ctx, identifier)
}

func (r *queryResolver) VulnerabilitySuppressions(ctx context.Context, filter *v0s.VulnerabilitySuppressionFilter, first *int, after *pagination.Cursor) (*pagination.Connection[*v0s.VulnerabilitySuppression], error) {
	return v0s.Suppressions(ctx, nil, filter, first, after)
}

func (r *vulnerabilityFindingResolver) Vulnerability(ctx context.Context, obj *v0s.VulnerabilityFinding) (*v0s.Vulnerability, error) {
	return v0s.VulnerabilityByIdentifier(ctx, obj.VulnerabilityIdentifier)
}

func (r *Resolver) VulnerabilityFinding() gengql.VulnerabilityFindingResolver {
	return &vulnerabilityFindingResolver{r}
}

type vulnerabilityFindingResolver struct{ *Resolver }
