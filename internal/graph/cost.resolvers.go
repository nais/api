package graph

import (
	"context"
	"errors"
	"time"

	"github.com/nais/api/internal/cost"
	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/scalar"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/persistence/bigquery"
	"github.com/nais/api/internal/persistence/opensearch"
	"github.com/nais/api/internal/persistence/sqlinstance"
	"github.com/nais/api/internal/persistence/valkey"
	"github.com/nais/api/internal/team"
	"github.com/nais/api/internal/workload"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
	"github.com/sirupsen/logrus"
)

func (r *applicationResolver) Cost(ctx context.Context, obj *application.Application) (*cost.WorkloadCost, error) {
	return &cost.WorkloadCost{
		EnvironmentName: obj.EnvironmentName,
		WorkloadName:    obj.Name,
		TeamSlug:        obj.TeamSlug,
	}, nil
}

func (r *bigQueryDatasetResolver) Cost(ctx context.Context, obj *bigquery.BigQueryDataset) (*cost.BigQueryDatasetCost, error) {
	if obj.WorkloadReference == nil {
		return &cost.BigQueryDatasetCost{}, nil
	}

	sum, err := cost.MonthlyForService(ctx, obj.TeamSlug, obj.EnvironmentName, obj.WorkloadReference.Name, "BigQuery")
	if err != nil {
		r.log.WithError(err).WithFields(logrus.Fields{
			"EnvironmentName": obj.EnvironmentName,
			"TeamSlug":        obj.TeamSlug,
			"BigQueryDataset": obj.Name,
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
	if obj.WorkloadReference == nil {
		return &cost.OpenSearchCost{}, nil
	}

	sum, err := cost.MonthlyForService(ctx, obj.TeamSlug, obj.EnvironmentName, obj.WorkloadReference.Name, "OpenSearch")
	if err != nil {
		r.log.WithError(err).WithFields(logrus.Fields{
			"EnvironmentName": obj.EnvironmentName,
			"TeamSlug":        obj.TeamSlug,
			"OpenSearch":      obj.Name,
		}).Warn("failed to get monthly cost for OpenSearch")
		return &cost.OpenSearchCost{
			Sum: 0,
		}, nil
	}

	return &cost.OpenSearchCost{
		Sum: float64(sum),
	}, nil
}

func (r *queryResolver) CostMonthlySummary(ctx context.Context, from scalar.Date, to scalar.Date) (*cost.CostMonthlySummary, error) {
	if to.Time().Before(from.Time()) {
		return nil, apierror.Errorf("`to` must be after `from`.")
	} else if to.Time().After(time.Now()) {
		return nil, apierror.Errorf("`to` cannot be in the future.")
	}
	return cost.MonthlySummaryForTenant(ctx, from.Time(), to.Time())
}

func (r *sqlInstanceResolver) Cost(ctx context.Context, obj *sqlinstance.SQLInstance) (*cost.SQLInstanceCost, error) {
	if obj.WorkloadReference == nil {
		return &cost.SQLInstanceCost{}, nil
	}

	sum, err := cost.MonthlyForService(ctx, obj.TeamSlug, obj.EnvironmentName, obj.WorkloadReference.Name, "Cloud SQL")
	if err != nil {
		return nil, nil
	}

	return &cost.SQLInstanceCost{
		Sum: float64(sum),
	}, nil
}

func (r *teamResolver) Cost(ctx context.Context, obj *team.Team) (*cost.TeamCost, error) {
	return &cost.TeamCost{TeamSlug: obj.Slug}, nil
}

func (r *teamCostResolver) Daily(ctx context.Context, obj *cost.TeamCost, from scalar.Date, to scalar.Date, filter *cost.TeamCostDailyFilter) (*cost.TeamCostPeriod, error) {
	if !to.Time().After(from.Time()) {
		return nil, apierror.Errorf("`to` must be after `from`.")
	} else if to.Time().After(time.Now()) {
		return nil, apierror.Errorf("`to` cannot be in the future.")
	}

	return cost.DailyForTeam(ctx, obj.TeamSlug, from.Time(), to.Time(), filter)
}

func (r *teamCostResolver) MonthlySummary(ctx context.Context, obj *cost.TeamCost) (*cost.TeamCostMonthlySummary, error) {
	return cost.MonthlySummaryForTeam(ctx, obj.TeamSlug)
}

func (r *teamEnvironmentResolver) Cost(ctx context.Context, obj *team.TeamEnvironment) (*cost.TeamEnvironmentCost, error) {
	return &cost.TeamEnvironmentCost{TeamSlug: obj.TeamSlug, EnvironmentName: obj.EnvironmentName}, nil
}

func (r *teamEnvironmentCostResolver) Daily(ctx context.Context, obj *cost.TeamEnvironmentCost, from scalar.Date, to scalar.Date) (*cost.TeamEnvironmentCostPeriod, error) {
	if !to.Time().After(from.Time()) {
		return nil, apierror.Errorf("`to` must be after `from`.")
	} else if to.Time().After(time.Now()) {
		return nil, apierror.Errorf("`to` cannot be in the future.")
	}

	return cost.DailyForTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName, from.Time(), to.Time())
}

func (r *valkeyInstanceResolver) Cost(ctx context.Context, obj *valkey.ValkeyInstance) (*cost.ValkeyInstanceCost, error) {
	if obj.WorkloadReference == nil {
		return &cost.ValkeyInstanceCost{}, nil
	}

	sum, err := cost.MonthlyForService(ctx, obj.TeamSlug, obj.EnvironmentName, obj.WorkloadReference.Name, "Valkey")
	if err != nil {
		r.log.WithError(err).WithFields(logrus.Fields{
			"EnvironmentName": obj.EnvironmentName,
			"TeamSlug":        obj.TeamSlug,
			"Valkey":          obj.Name,
		}).Warn("failed to get monthly cost for Valkey instance")
		return &cost.ValkeyInstanceCost{
			Sum: 0,
		}, nil
	}

	return &cost.ValkeyInstanceCost{
		Sum: float64(sum),
	}, nil
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

func (r *workloadCostSampleResolver) Workload(ctx context.Context, obj *cost.WorkloadCostSample) (workload.Workload, error) {
	w, err := tryWorkload(ctx, obj.TeamSlug, obj.EnvironmentName, obj.WorkloadName)
	if errors.Is(err, &watcher.ErrorNotFound{}) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	return w, nil
}

func (r *Resolver) TeamCost() gengql.TeamCostResolver { return &teamCostResolver{r} }

func (r *Resolver) TeamEnvironmentCost() gengql.TeamEnvironmentCostResolver {
	return &teamEnvironmentCostResolver{r}
}

func (r *Resolver) WorkloadCost() gengql.WorkloadCostResolver { return &workloadCostResolver{r} }

func (r *Resolver) WorkloadCostSample() gengql.WorkloadCostSampleResolver {
	return &workloadCostSampleResolver{r}
}

type (
	teamCostResolver            struct{ *Resolver }
	teamEnvironmentCostResolver struct{ *Resolver }
	workloadCostResolver        struct{ *Resolver }
	workloadCostSampleResolver  struct{ *Resolver }
)
