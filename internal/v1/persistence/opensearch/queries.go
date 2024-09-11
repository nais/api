package opensearch

import (
	"context"
	"slices"

	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v1/graphv1/ident"
	"github.com/nais/api/internal/v1/graphv1/modelv1"
	"github.com/nais/api/internal/v1/graphv1/pagination"
	"github.com/nais/api/internal/v1/kubernetes/watcher"
	nais_io_v1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
)

func GetByIdent(ctx context.Context, id ident.Ident) (*OpenSearch, error) {
	teamSlug, environment, name, err := parseIdent(id)
	if err != nil {
		return nil, err
	}

	return Get(ctx, teamSlug, environment, name)
}

func Get(ctx context.Context, teamSlug slug.Slug, environment, name string) (*OpenSearch, error) {
	return fromContext(ctx).client.watcher.Get(environment, teamSlug.String(), name)
}

func ListForTeam(ctx context.Context, teamSlug slug.Slug, page *pagination.Pagination, orderBy *OpenSearchOrder) (*OpenSearchConnection, error) {
	all := fromContext(ctx).client.watcher.GetByNamespace(teamSlug.String())
	ret := watcher.Objects(all)

	orderOpenSearch(ret, orderBy)

	instances := pagination.Slice(ret, page)
	return pagination.NewConnection(instances, page, int32(len(ret))), nil
}

func ListAccess(ctx context.Context, openSearch *OpenSearch, page *pagination.Pagination, orderBy *OpenSearchAccessOrder) (*OpenSearchAccessConnection, error) {
	k8sClient := fromContext(ctx).client

	applicationAccess, err := k8sClient.getAccessForApplications(ctx, openSearch.EnvironmentName, openSearch.Name, openSearch.TeamSlug)
	if err != nil {
		return nil, err
	}

	jobAccess, err := k8sClient.getAccessForJobs(ctx, openSearch.EnvironmentName, openSearch.Name, openSearch.TeamSlug)
	if err != nil {
		return nil, err
	}

	all := make([]*OpenSearchAccess, 0)
	all = append(all, applicationAccess...)
	all = append(all, jobAccess...)

	if orderBy != nil {
		switch orderBy.Field {
		case OpenSearchAccessOrderFieldAccess:
			slices.SortStableFunc(all, func(a, b *OpenSearchAccess) int {
				return modelv1.Compare(a.Access, b.Access, orderBy.Direction)
			})
		case OpenSearchAccessOrderFieldWorkload:
			slices.SortStableFunc(all, func(a, b *OpenSearchAccess) int {
				return modelv1.Compare(a.WorkloadReference.Name, b.WorkloadReference.Name, orderBy.Direction)
			})
		}
	}

	ret := pagination.Slice(all, page)
	return pagination.NewConnection(ret, page, int32(len(all))), nil
}

func GetForWorkload(ctx context.Context, teamSlug slug.Slug, environment string, reference *nais_io_v1.OpenSearch) (*OpenSearch, error) {
	if reference == nil {
		return nil, nil
	}

	return fromContext(ctx).client.watcher.Get(environment, teamSlug.String(), openSearchNamer(teamSlug, reference.Instance))
}

func orderOpenSearch(ret []*OpenSearch, orderBy *OpenSearchOrder) {
	if orderBy != nil {
		switch orderBy.Field {
		case OpenSearchOrderFieldName:
			slices.SortStableFunc(ret, func(a, b *OpenSearch) int {
				return modelv1.Compare(a.Name, b.Name, orderBy.Direction)
			})
		case OpenSearchOrderFieldEnvironment:
			slices.SortStableFunc(ret, func(a, b *OpenSearch) int {
				return modelv1.Compare(a.EnvironmentName, b.EnvironmentName, orderBy.Direction)
			})
		}
	}
}
