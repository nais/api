package graph

import (
	"context"
	"errors"

	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/persistence/bigquery"
	"github.com/nais/api/internal/team"
	"github.com/nais/api/internal/workload"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
)

func (r *applicationResolver) BigQueryDatasets(ctx context.Context, obj *application.Application, orderBy *bigquery.BigQueryDatasetOrder) (*pagination.Connection[*bigquery.BigQueryDataset], error) {
	if obj.Spec.GCP == nil {
		return pagination.EmptyConnection[*bigquery.BigQueryDataset](), nil
	}

	return bigquery.ListForWorkload(ctx, obj.TeamSlug, obj.Spec.GCP.BigQueryDatasets, orderBy)
}

func (r *bigQueryDatasetResolver) Team(ctx context.Context, obj *bigquery.BigQueryDataset) (*team.Team, error) {
	return team.Get(ctx, obj.TeamSlug)
}

func (r *bigQueryDatasetResolver) Environment(ctx context.Context, obj *bigquery.BigQueryDataset) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
}

func (r *bigQueryDatasetResolver) Access(ctx context.Context, obj *bigquery.BigQueryDataset, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *bigquery.BigQueryDatasetAccessOrder) (*pagination.Connection[*bigquery.BigQueryDatasetAccess], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	if orderBy == nil {
		orderBy = &bigquery.BigQueryDatasetAccessOrder{
			Field:     "EMAIL",
			Direction: model.OrderDirectionAsc,
		}
	}

	bigquery.SortFilterAccess.Sort(ctx, obj.Access, orderBy.Field, orderBy.Direction)

	ret := pagination.Slice(obj.Access, page)
	return pagination.NewConnection(ret, page, len(obj.Access)), nil
}

func (r *bigQueryDatasetResolver) Workload(ctx context.Context, obj *bigquery.BigQueryDataset) (workload.Workload, error) {
	w, err := getWorkload(ctx, obj.WorkloadReference, obj.TeamSlug, obj.EnvironmentName)
	if errors.Is(err, &watcher.ErrorNotFound{}) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return w, nil
}

func (r *jobResolver) BigQueryDatasets(ctx context.Context, obj *job.Job, orderBy *bigquery.BigQueryDatasetOrder) (*pagination.Connection[*bigquery.BigQueryDataset], error) {
	if obj.Spec.GCP == nil {
		return pagination.EmptyConnection[*bigquery.BigQueryDataset](), nil
	}

	return bigquery.ListForWorkload(ctx, obj.TeamSlug, obj.Spec.GCP.BigQueryDatasets, orderBy)
}

func (r *teamResolver) BigQueryDatasets(ctx context.Context, obj *team.Team, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *bigquery.BigQueryDatasetOrder) (*pagination.Connection[*bigquery.BigQueryDataset], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return bigquery.ListForTeam(ctx, obj.Slug, page, orderBy)
}

func (r *teamEnvironmentResolver) BigQueryDataset(ctx context.Context, obj *team.TeamEnvironment, name string) (*bigquery.BigQueryDataset, error) {
	return bigquery.Get(ctx, obj.TeamSlug, obj.Name, name)
}

func (r *teamInventoryCountsResolver) BigQueryDatasets(ctx context.Context, obj *team.TeamInventoryCounts) (*bigquery.TeamInventoryCountBigQueryDatasets, error) {
	return &bigquery.TeamInventoryCountBigQueryDatasets{
		Total: len(bigquery.ListAllForTeam(ctx, obj.TeamSlug)),
	}, nil
}

func (r *Resolver) BigQueryDataset() gengql.BigQueryDatasetResolver {
	return &bigQueryDatasetResolver{r}
}

type bigQueryDatasetResolver struct{ *Resolver }
