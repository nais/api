package bigquery

import (
	"context"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/search"
	"github.com/nais/api/internal/slug"
	nais_io_v1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
)

func GetByIdent(ctx context.Context, id ident.Ident) (*BigQueryDataset, error) {
	teamSlug, environment, name, err := parseIdent(id)
	if err != nil {
		return nil, err
	}

	return fromContext(ctx).watcher.Get(environment, teamSlug.String(), name)
}

func Get(ctx context.Context, teamSlug slug.Slug, environment, name string) (*BigQueryDataset, error) {
	all := fromContext(ctx).watcher.GetByNamespace(teamSlug.String(), watcher.InCluster(environment))
	for _, dataset := range all {
		if dataset.Obj.Name == name {
			return dataset.Obj, nil
		}
	}

	return nil, &watcher.ErrorNotFound{
		Cluster:   environment,
		Name:      name,
		Namespace: teamSlug.String(),
	}
}

func ListForTeam(ctx context.Context, teamSlug slug.Slug, page *pagination.Pagination, orderBy *BigQueryDatasetOrder) (*BigQueryDatasetConnection, error) {
	all := ListAllForTeam(ctx, teamSlug)
	orderDatasets(ctx, all, orderBy)

	datasets := pagination.Slice(all, page)
	return pagination.NewConnection(datasets, page, len(all)), nil
}

func ListAllForTeam(ctx context.Context, teamSlug slug.Slug) []*BigQueryDataset {
	all := fromContext(ctx).watcher.GetByNamespace(teamSlug.String())
	return watcher.Objects(all)
}

func ListForWorkload(ctx context.Context, teamSlug slug.Slug, datasets []nais_io_v1.CloudBigQueryDataset, orderBy *BigQueryDatasetOrder) (*BigQueryDatasetConnection, error) {
	all := fromContext(ctx).watcher.GetByNamespace(teamSlug.String())
	ret := make([]*BigQueryDataset, 0)

	for _, dataset := range datasets {
		for _, d := range all {
			if d.Obj.Name == dataset.Name {
				ret = append(ret, d.Obj)
			}
		}
	}

	orderDatasets(ctx, ret, orderBy)
	return pagination.NewConnectionWithoutPagination(ret), nil
}

func Search(ctx context.Context, q string) ([]*search.Result, error) {
	apps := fromContext(ctx).watcher.All()

	ret := make([]*search.Result, 0)
	for _, app := range apps {
		rank := search.Match(q, app.Obj.Name)
		if search.Include(rank) {
			ret = append(ret, &search.Result{
				Rank: rank,
				Node: app.Obj,
			})
		}
	}

	return ret, nil
}

func orderDatasets(ctx context.Context, datasets []*BigQueryDataset, orderBy *BigQueryDatasetOrder) {
	if orderBy == nil {
		orderBy = &BigQueryDatasetOrder{
			Field:     BigQueryDatasetOrderFieldName,
			Direction: model.OrderDirectionAsc,
		}
	}
	SortFilter.Sort(ctx, datasets, orderBy.Field, orderBy.Direction)
}
