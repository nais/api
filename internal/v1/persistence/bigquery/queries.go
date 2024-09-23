package bigquery

import (
	"context"
	"slices"

	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v1/graphv1/ident"
	"github.com/nais/api/internal/v1/graphv1/modelv1"
	"github.com/nais/api/internal/v1/graphv1/pagination"
	"github.com/nais/api/internal/v1/kubernetes/watcher"
	"github.com/nais/api/internal/v1/searchv1"
	nais_io_v1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
)

func GetByIdent(ctx context.Context, id ident.Ident) (*BigQueryDataset, error) {
	teamSlug, environment, name, err := parseIdent(id)
	if err != nil {
		return nil, err
	}

	return Get(ctx, teamSlug, environment, name)
}

func Get(ctx context.Context, teamSlug slug.Slug, environment, name string) (*BigQueryDataset, error) {
	return fromContext(ctx).datasetLoader.Load(ctx, resourceIdentifier{
		namespace:   teamSlug.String(),
		environment: environment,
		name:        name,
	})
}

func ListForTeam(ctx context.Context, teamSlug slug.Slug, page *pagination.Pagination, orderBy *BigQueryDatasetOrder) (*BigQueryDatasetConnection, error) {
	all := fromContext(ctx).watcher.GetByNamespace(teamSlug.String())
	ret := watcher.Objects(all)

	orderDatasets(ret, orderBy)

	datasets := pagination.Slice(ret, page)
	return pagination.NewConnection(datasets, page, int32(len(ret))), nil
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

	orderDatasets(ret, orderBy)
	return pagination.NewConnectionWithoutPagination(ret), nil
}

func Search(ctx context.Context, q string) ([]*searchv1.Result, error) {
	apps := fromContext(ctx).watcher.All()

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

func orderDatasets(datasets []*BigQueryDataset, orderBy *BigQueryDatasetOrder) {
	if orderBy == nil {
		orderBy = &BigQueryDatasetOrder{
			Field:     BigQueryDatasetOrderFieldName,
			Direction: modelv1.OrderDirectionAsc,
		}
	}
	switch orderBy.Field {
	case BigQueryDatasetOrderFieldName:
		slices.SortStableFunc(datasets, func(a, b *BigQueryDataset) int {
			return modelv1.Compare(a.Name, b.Name, orderBy.Direction)
		})
	case BigQueryDatasetOrderFieldEnvironment:
		slices.SortStableFunc(datasets, func(a, b *BigQueryDataset) int {
			return modelv1.Compare(a.EnvironmentName, b.EnvironmentName, orderBy.Direction)
		})
	}
}
