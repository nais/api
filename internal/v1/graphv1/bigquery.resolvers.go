package graphv1

import (
	"context"
	"slices"

	"github.com/nais/api/internal/v1/graphv1/gengqlv1"
	"github.com/nais/api/internal/v1/graphv1/modelv1"
	"github.com/nais/api/internal/v1/graphv1/pagination"
	"github.com/nais/api/internal/v1/persistence/bigquery"
	"github.com/nais/api/internal/v1/team"
	"github.com/nais/api/internal/v1/workload"
	"github.com/nais/api/internal/v1/workload/application"
	"github.com/nais/api/internal/v1/workload/job"
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

	if orderBy != nil {
		switch orderBy.Field {
		case bigquery.BigQueryDatasetAccessOrderFieldRole:
			slices.SortStableFunc(obj.Access, func(a, b *bigquery.BigQueryDatasetAccess) int {
				return modelv1.Compare(a.Role, b.Role, orderBy.Direction)
			})
		case bigquery.BigQueryDatasetAccessOrderFieldEmail:
			slices.SortStableFunc(obj.Access, func(a, b *bigquery.BigQueryDatasetAccess) int {
				return modelv1.Compare(a.Email, b.Email, orderBy.Direction)
			})

		}
	}

	ret := pagination.Slice(obj.Access, page)
	return pagination.NewConnection(ret, page, int32(len(obj.Access))), nil
}

func (r *bigQueryDatasetResolver) Workload(ctx context.Context, obj *bigquery.BigQueryDataset) (workload.Workload, error) {
	return getWorkload(ctx, obj.WorkloadReference, obj.TeamSlug, obj.EnvironmentName)
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

func (r *Resolver) BigQueryDataset() gengqlv1.BigQueryDatasetResolver {
	return &bigQueryDatasetResolver{r}
}

type bigQueryDatasetResolver struct{ *Resolver }