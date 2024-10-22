package graphv1

import (
	"context"
	"time"

	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/v1/cost"
	"github.com/nais/api/internal/v1/graphv1/gengqlv1"
	"github.com/nais/api/internal/v1/graphv1/scalar"
	"github.com/nais/api/internal/v1/persistence/bigquery"
	"github.com/nais/api/internal/v1/persistence/opensearch"
	"github.com/nais/api/internal/v1/persistence/redis"
	"github.com/nais/api/internal/v1/team"
	"github.com/nais/api/internal/v1/workload/application"
	"github.com/nais/api/internal/v1/workload/job"
	"github.com/sirupsen/logrus"
)

func (r *applicationResolver) Cost(_ context.Context, obj *application.Application) (*cost.WorkloadCost, error) {
	return &cost.WorkloadCost{
		EnvironmentName: obj.EnvironmentName,
		WorkloadName:    obj.Name,
		TeamSlug:        obj.TeamSlug,
	}, nil
}

func (r *bigQueryDatasetResolver) Cost(ctx context.Context, obj *bigquery.BigQueryDataset) (*cost.BigQueryDatasetCost, error) {
	sum, err := cost.MonthlyForService(ctx, obj.TeamSlug, obj.EnvironmentName, obj.WorkloadReference.Name, "BigQuery")
	if err != nil {
		r.log.WithError(err).WithFields(logrus.Fields{
			"EnvironmentName": obj.EnvironmentName,
			"WorkloadName":    obj.Name,
			"TeamSlug":        obj.TeamSlug,
		}).Warn("failed to get monthly cost for BigQuery dataset")
		return &cost.BigQueryDatasetCost{
			Sum: 0,
		}, nil
	}

	return &cost.BigQueryDatasetCost{
		Sum: float64(sum),
	}, nil
}

func (r *jobResolver) Cost(ctx context.Context, obj *job.Job) (*cost.WorkloadCost, error) {
	return &cost.WorkloadCost{
		EnvironmentName: obj.EnvironmentName,
		WorkloadName:    obj.Name,
		TeamSlug:        obj.TeamSlug,
	}, nil
}

func (r *openSearchResolver) Cost(ctx context.Context, obj *opensearch.OpenSearch) (*cost.OpenSearchCost, error) {
	sum, err := cost.MonthlyForService(ctx, obj.TeamSlug, obj.EnvironmentName, obj.WorkloadReference.Name, "Redis")
	if err != nil {
		r.log.WithError(err).WithFields(logrus.Fields{
			"EnvironmentName": obj.EnvironmentName,
			"WorkloadName":    obj.Name,
			"TeamSlug":        obj.TeamSlug,
		}).Warn("failed to get monthly cost for OpenSearch")
		return &cost.OpenSearchCost{
			Sum: 0,
		}, nil
	}

	return &cost.OpenSearchCost{
		Sum: float64(sum),
	}, nil
}

func (r *redisInstanceResolver) Cost(ctx context.Context, obj *redis.RedisInstance) (*cost.RedisInstanceCost, error) {
	sum, err := cost.MonthlyForService(ctx, obj.TeamSlug, obj.EnvironmentName, obj.WorkloadReference.Name, "Redis")
	if err != nil {
		r.log.WithError(err).WithFields(logrus.Fields{
			"EnvironmentName": obj.EnvironmentName,
			"WorkloadName":    obj.Name,
			"TeamSlug":        obj.TeamSlug,
		}).Warn("failed to get monthly cost for Redis instance")
		return &cost.RedisInstanceCost{
			Sum: 0,
		}, nil
	}

	return &cost.RedisInstanceCost{
		Sum: float64(sum),
	}, nil
}

func (r *teamResolver) Cost(ctx context.Context, obj *team.Team) (*cost.TeamCost, error) {
	return &cost.TeamCost{TeamSlug: obj.Slug}, nil
}

func (r *teamCostResolver) Daily(ctx context.Context, obj *cost.TeamCost, from scalar.Date, to scalar.Date) (*cost.TeamCostPeriod, error) {
	if !to.Time().After(from.Time()) {
		return nil, apierror.Errorf("`to` must be after `from`.")
	} else if to.Time().After(time.Now()) {
		return nil, apierror.Errorf("`to` cannot be in the future.")
	}

	return cost.DailyForTeam(ctx, obj.TeamSlug, from.Time(), to.Time())
}

func (r *teamCostResolver) MonthlySummary(ctx context.Context, obj *cost.TeamCost) (*cost.TeamCostMonthlySummary, error) {
	return cost.MonthlySummaryForTeam(ctx, obj.TeamSlug)
}

func (r *workloadCostResolver) Daily(ctx context.Context, obj *cost.WorkloadCost, from scalar.Date, to scalar.Date) (*cost.WorkloadCostPeriod, error) {
	if !to.Time().After(from.Time()) {
		return nil, apierror.Errorf("`to` must be after `from`.")
	} else if to.Time().After(time.Now()) {
		return nil, apierror.Errorf("`to` cannot be in the future.")
	}

	return cost.DailyForWorkload(ctx, obj.TeamSlug, obj.EnvironmentName, obj.WorkloadName, from.Time(), to.Time())
}

func (r *workloadCostResolver) Monthly(ctx context.Context, obj *cost.WorkloadCost) (*cost.WorkloadCostPeriod, error) {
	return cost.MonthlyForWorkload(ctx, obj.TeamSlug, obj.EnvironmentName, obj.WorkloadName)
}

func (r *Resolver) TeamCost() gengqlv1.TeamCostResolver { return &teamCostResolver{r} }

func (r *Resolver) WorkloadCost() gengqlv1.WorkloadCostResolver { return &workloadCostResolver{r} }

type (
	teamCostResolver     struct{ *Resolver }
	workloadCostResolver struct{ *Resolver }
)
