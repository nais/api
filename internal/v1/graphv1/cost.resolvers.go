package graphv1

import (
	"context"
	"time"

	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/v1/cost"
	"github.com/nais/api/internal/v1/graphv1/gengqlv1"
	"github.com/nais/api/internal/v1/graphv1/scalar"
	"github.com/nais/api/internal/v1/team"
	"github.com/nais/api/internal/v1/workload/application"
	"github.com/nais/api/internal/v1/workload/job"
)

func (r *applicationResolver) Cost(ctx context.Context, obj *application.Application) (*cost.WorkloadCost, error) {
	return &cost.WorkloadCost{
		EnvironmentName: obj.EnvironmentName,
		WorkloadName:    obj.Name,
		TeamSlug:        obj.TeamSlug,
	}, nil
}

func (r *jobResolver) Cost(ctx context.Context, obj *job.Job) (*cost.WorkloadCost, error) {
	return &cost.WorkloadCost{
		EnvironmentName: obj.EnvironmentName,
		WorkloadName:    obj.Name,
		TeamSlug:        obj.TeamSlug,
	}, nil
}

func (r *teamResolver) Cost(ctx context.Context, obj *team.Team) (*cost.TeamCost, error) {
	return &cost.TeamCost{TeamSlug: obj.Slug}, nil
}

func (r *teamCostResolver) Daily(ctx context.Context, obj *cost.TeamCost, from scalar.Date, to scalar.Date) (*cost.TeamCostPeriod, error) {
	if to.Time().Before(from.Time()) {
		return nil, apierror.Errorf("to date cannot be before from date")
	} else if to.Time().After(time.Now()) {
		return nil, apierror.Errorf("to date cannot be in the future")
	}

	return cost.DailyForTeam(ctx, obj.TeamSlug, from.Time(), to.Time())
}

func (r *teamCostResolver) MonthlySummary(ctx context.Context, obj *cost.TeamCost) (*cost.TeamCostMonthlySummary, error) {
	return cost.MonthlySummaryForTeam(ctx, obj.TeamSlug)
}

func (r *workloadCostResolver) Daily(ctx context.Context, obj *cost.WorkloadCost, from scalar.Date, to scalar.Date) (*cost.WorkloadCostPeriod, error) {
	if to.Time().Before(from.Time()) {
		return nil, apierror.Errorf("to date cannot be before from date")
	} else if to.Time().After(time.Now()) {
		return nil, apierror.Errorf("to date cannot be in the future")
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