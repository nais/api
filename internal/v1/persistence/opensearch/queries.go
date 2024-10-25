package opensearch

import (
	"context"

	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v1/graphv1/ident"
	"github.com/nais/api/internal/v1/graphv1/modelv1"
	"github.com/nais/api/internal/v1/graphv1/pagination"
	"github.com/nais/api/internal/v1/kubernetes/watcher"
	"github.com/nais/api/internal/v1/searchv1"
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
	all := ListAllForTeam(ctx, teamSlug)
	orderOpenSearch(ctx, all, orderBy)

	instances := pagination.Slice(all, page)
	return pagination.NewConnection(instances, page, int32(len(all))), nil
}

func ListAllForTeam(ctx context.Context, teamSlug slug.Slug) []*OpenSearch {
	all := fromContext(ctx).client.watcher.GetByNamespace(teamSlug.String())
	return watcher.Objects(all)
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

	if orderBy == nil {
		orderBy = &OpenSearchAccessOrder{Field: OpenSearchAccessOrderFieldAccess, Direction: modelv1.OrderDirectionAsc}
	}
	SortFilterOpenSearchAccess.Sort(ctx, all, orderBy.Field, orderBy.Direction)

	ret := pagination.Slice(all, page)
	return pagination.NewConnection(ret, page, int32(len(all))), nil
}

func GetForWorkload(ctx context.Context, teamSlug slug.Slug, environment string, reference *nais_io_v1.OpenSearch) (*OpenSearch, error) {
	if reference == nil {
		return nil, nil
	}

	return fromContext(ctx).client.watcher.Get(environment, teamSlug.String(), openSearchNamer(teamSlug, reference.Instance))
}

func Search(ctx context.Context, q string) ([]*searchv1.Result, error) {
	apps := fromContext(ctx).client.watcher.All()

	ret := make([]*searchv1.Result, 0)
	for _, app := range apps {
		rank := searchv1.Match(q, app.Obj.Name)
		if searchv1.Include(rank) {
			ret = append(ret, &searchv1.Result{
				Rank: rank,
				Node: app.Obj,
			})
		}
	}

	return ret, nil
}

func orderOpenSearch(ctx context.Context, ret []*OpenSearch, orderBy *OpenSearchOrder) {
	if orderBy == nil {
		orderBy = &OpenSearchOrder{Field: OpenSearchOrderFieldName, Direction: modelv1.OrderDirectionAsc}
	}

	SortFilterOpenSearch.Sort(ctx, ret, orderBy.Field, orderBy.Direction)
}
