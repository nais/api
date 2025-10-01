package graph

import (
	"context"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/issue"
	"github.com/nais/api/internal/persistence/opensearch"
	"github.com/nais/api/internal/team"
	"github.com/nais/api/internal/workload"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
)

func (r *applicationResolver) OpenSearch(ctx context.Context, obj *application.Application) (*opensearch.OpenSearch, error) {
	return opensearch.GetForWorkload(ctx, obj.TeamSlug, obj.EnvironmentName, obj.Spec.OpenSearch)
}

func (r *jobResolver) OpenSearch(ctx context.Context, obj *job.Job) (*opensearch.OpenSearch, error) {
	return opensearch.GetForWorkload(ctx, obj.TeamSlug, obj.EnvironmentName, obj.Spec.OpenSearch)
}

func (r *mutationResolver) CreateOpenSearch(ctx context.Context, input opensearch.CreateOpenSearchInput) (*opensearch.CreateOpenSearchPayload, error) {
	if err := authz.CanCreateOpenSearch(ctx, input.TeamSlug); err != nil {
		return nil, err
	}
	return opensearch.Create(ctx, input)
}

func (r *mutationResolver) UpdateOpenSearch(ctx context.Context, input opensearch.UpdateOpenSearchInput) (*opensearch.UpdateOpenSearchPayload, error) {
	if err := authz.CanUpdateOpenSearch(ctx, input.TeamSlug); err != nil {
		return nil, err
	}
	return opensearch.Update(ctx, input)
}

func (r *mutationResolver) DeleteOpenSearch(ctx context.Context, input opensearch.DeleteOpenSearchInput) (*opensearch.DeleteOpenSearchPayload, error) {
	if err := authz.CanDeleteOpenSearch(ctx, input.TeamSlug); err != nil {
		return nil, err
	}
	return opensearch.Delete(ctx, input)
}

func (r *openSearchResolver) Team(ctx context.Context, obj *opensearch.OpenSearch) (*team.Team, error) {
	return team.Get(ctx, obj.TeamSlug)
}

func (r *openSearchResolver) Environment(ctx context.Context, obj *opensearch.OpenSearch) (*team.TeamEnvironment, error) {
	return r.TeamEnvironment(ctx, obj)
}

func (r *openSearchResolver) TeamEnvironment(ctx context.Context, obj *opensearch.OpenSearch) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
}

func (r *openSearchResolver) State(ctx context.Context, obj *opensearch.OpenSearch) (opensearch.OpenSearchState, error) {
	return opensearch.State(ctx, obj)
}

func (r *openSearchResolver) Workload(ctx context.Context, obj *opensearch.OpenSearch) (workload.Workload, error) {
	return getWorkload(ctx, obj.WorkloadReference, obj.TeamSlug, obj.EnvironmentName)
}

func (r *openSearchResolver) Access(ctx context.Context, obj *opensearch.OpenSearch, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *opensearch.OpenSearchAccessOrder) (*pagination.Connection[*opensearch.OpenSearchAccess], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return opensearch.ListAccess(ctx, obj, page, orderBy)
}

func (r *openSearchResolver) Version(ctx context.Context, obj *opensearch.OpenSearch) (*opensearch.OpenSearchVersion, error) {
	return opensearch.GetOpenSearchVersion(ctx, obj)
}

func (r *openSearchResolver) Issues(ctx context.Context, obj *opensearch.OpenSearch, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *issue.IssueOrder, filter *issue.IssueFilter) (*pagination.Connection[issue.Issue], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	t := issue.ResourceTypeOpensearch
	f := &issue.IssueFilter{
		ResourceName: &obj.Name,
		ResourceType: &t,
		Environments: []string{obj.EnvironmentName},
	}
	if filter != nil {
		f.Severity = filter.Severity
		f.IssueType = filter.IssueType
	}

	return issue.ListIssues(ctx, obj.TeamSlug, page, orderBy, f)
}

func (r *openSearchAccessResolver) Workload(ctx context.Context, obj *opensearch.OpenSearchAccess) (workload.Workload, error) {
	return getWorkload(ctx, obj.WorkloadReference, obj.TeamSlug, obj.EnvironmentName)
}

func (r *teamResolver) OpenSearches(ctx context.Context, obj *team.Team, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *opensearch.OpenSearchOrder) (*pagination.Connection[*opensearch.OpenSearch], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return opensearch.ListForTeam(ctx, obj.Slug, page, orderBy)
}

func (r *teamEnvironmentResolver) OpenSearch(ctx context.Context, obj *team.TeamEnvironment, name string) (*opensearch.OpenSearch, error) {
	return opensearch.Get(ctx, obj.TeamSlug, obj.EnvironmentName, name)
}

func (r *teamInventoryCountsResolver) OpenSearches(ctx context.Context, obj *team.TeamInventoryCounts) (*opensearch.TeamInventoryCountOpenSearches, error) {
	return &opensearch.TeamInventoryCountOpenSearches{
		Total: len(opensearch.ListAllForTeam(ctx, obj.TeamSlug)),
	}, nil
}

func (r *Resolver) OpenSearch() gengql.OpenSearchResolver { return &openSearchResolver{r} }

func (r *Resolver) OpenSearchAccess() gengql.OpenSearchAccessResolver {
	return &openSearchAccessResolver{r}
}

type (
	openSearchResolver       struct{ *Resolver }
	openSearchAccessResolver struct{ *Resolver }
)
