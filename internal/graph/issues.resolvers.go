package graph

import (
	"context"
	"fmt"

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

func (r *deprecatedIngressIssueResolver) Application(ctx context.Context, obj *issue.DeprecatedIngressIssue) (*application.Application, error) {
	return application.Get(ctx, obj.Team, obj.Environment, obj.ResourceName)
}

func (r *deprecatedRegistryIssueResolver) Workload(ctx context.Context, obj *issue.DeprecatedRegistryIssue) (workload.Workload, error) {
	switch obj.ResourceType {
	case issue.ResourceTypeApplication:
		return application.Get(ctx, obj.Team, obj.Environment, obj.ResourceName)
	case issue.ResourceTypeJob:
		return job.Get(ctx, obj.Team, obj.Environment, obj.ResourceName)
	default:
		return nil, fmt.Errorf("unknown resource type: %s", obj.ResourceType)
	}
}

func (r *openSearchIssueResolver) OpenSearch(ctx context.Context, obj *issue.OpenSearchIssue) (*opensearch.OpenSearch, error) {
	return opensearch.Get(ctx, obj.Team, obj.Environment, obj.ResourceName)
}

func (r *sqlInstanceStateIssueResolver) SQLInstance(ctx context.Context, obj *issue.SqlInstanceStateIssue) (*sqlinstance.SQLInstance, error) {
	return sqlinstance.Get(ctx, obj.Team, obj.Environment, obj.ResourceName)
}

func (r *sqlInstanceVersionIssueResolver) SQLInstance(ctx context.Context, obj *issue.SqlInstanceVersionIssue) (*sqlinstance.SQLInstance, error) {
	return sqlinstance.Get(ctx, obj.Team, obj.Environment, obj.ResourceName)
}

func (r *teamResolver) Issues(ctx context.Context, obj *team.Team, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *issue.IssueOrder, filter *issue.IssueFilter) (*pagination.Connection[issue.Issue], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return issue.ListForTeam(ctx, obj.Slug, page, orderBy, filter)
}

func (r *valkeyIssueResolver) Valkey(ctx context.Context, obj *issue.ValkeyIssue) (*valkey.Valkey, error) {
	return valkey.Get(ctx, obj.Team, obj.Environment, obj.ResourceName)
}

func (r *Resolver) DeprecatedIngressIssue() gengql.DeprecatedIngressIssueResolver {
	return &deprecatedIngressIssueResolver{r}
}

func (r *Resolver) DeprecatedRegistryIssue() gengql.DeprecatedRegistryIssueResolver {
	return &deprecatedRegistryIssueResolver{r}
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
	deprecatedIngressIssueResolver  struct{ *Resolver }
	deprecatedRegistryIssueResolver struct{ *Resolver }
	openSearchIssueResolver         struct{ *Resolver }
	sqlInstanceStateIssueResolver   struct{ *Resolver }
	sqlInstanceVersionIssueResolver struct{ *Resolver }
	valkeyIssueResolver             struct{ *Resolver }
)
