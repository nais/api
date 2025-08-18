package graph

import (
	"context"

	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/pagination"
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

func (r *openSearchResolver) Team(ctx context.Context, obj *opensearch.OpenSearch) (*team.Team, error) {
	return team.Get(ctx, obj.TeamSlug)
}

func (r *openSearchResolver) Environment(ctx context.Context, obj *opensearch.OpenSearch) (*team.TeamEnvironment, error) {
	return r.TeamEnvironment(ctx, obj)
}

func (r *openSearchResolver) TeamEnvironment(ctx context.Context, obj *opensearch.OpenSearch) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
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

func (r *openSearchResolver) Version(ctx context.Context, obj *opensearch.OpenSearch) (string, error) {
	return opensearch.GetOpenSearchVersion(ctx, opensearch.AivenDataLoaderKey{
		Project:     obj.AivenProject,
		ServiceName: obj.Name,
	})
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
