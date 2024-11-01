package graphv1

import (
	"context"

	"github.com/nais/api/internal/v1/graphv1/gengqlv1"
	"github.com/nais/api/internal/v1/team"
	"github.com/nais/api/internal/v1/utilization"
	"github.com/nais/api/internal/v1/workload"
	"github.com/nais/api/internal/v1/workload/application"
)

func (r *applicationResolver) Utilization(ctx context.Context, obj *application.Application) (*utilization.WorkloadUtilization, error) {
	return &utilization.WorkloadUtilization{
		EnvironmentName: obj.EnvironmentName,
		WorkloadName:    obj.Name,
		TeamSlug:        obj.TeamSlug,
		WorkloadType:    utilization.WorkloadTypeApplication,
	}, nil
}

func (r *queryResolver) TeamsUtilization(ctx context.Context, resourceType utilization.UtilizationResourceType) ([]*utilization.TeamUtilizationData, error) {
	return utilization.ForTeams(ctx, resourceType)
}

func (r *teamResolver) WorkloadUtilization(ctx context.Context, obj *team.Team, resourceType utilization.UtilizationResourceType) ([]*utilization.WorkloadUtilizationData, error) {
	return utilization.ForTeam(ctx, obj.Slug, resourceType)
}

func (r *teamResolver) ServiceUtilization(ctx context.Context, obj *team.Team) (*utilization.TeamServiceUtilization, error) {
	return &utilization.TeamServiceUtilization{
		TeamSlug: obj.Slug,
	}, nil
}

func (r *teamUtilizationDataResolver) Team(ctx context.Context, obj *utilization.TeamUtilizationData) (*team.Team, error) {
	return team.Get(ctx, obj.TeamSlug)
}

func (r *teamUtilizationDataResolver) Environment(ctx context.Context, obj *utilization.TeamUtilizationData) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
}

func (r *teamUtilizationEnvironmentDataPointResolver) Environment(ctx context.Context, obj *utilization.TeamUtilizationEnvironmentDataPoint) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
}

func (r *workloadUtilizationResolver) Current(ctx context.Context, obj *utilization.WorkloadUtilization, resourceType utilization.UtilizationResourceType) (float64, error) {
	return utilization.WorkloadResourceUsage(ctx, obj.EnvironmentName, obj.TeamSlug, obj.WorkloadName, resourceType)
}

func (r *workloadUtilizationResolver) Requested(ctx context.Context, obj *utilization.WorkloadUtilization, resourceType utilization.UtilizationResourceType) (float64, error) {
	return utilization.WorkloadResourceRequest(ctx, obj.EnvironmentName, obj.TeamSlug, obj.WorkloadName, resourceType)
}

func (r *workloadUtilizationResolver) Series(ctx context.Context, obj *utilization.WorkloadUtilization, input utilization.WorkloadUtilizationSeriesInput) ([]*utilization.UtilizationDataPoint, error) {
	return utilization.WorkloadResourceUsageRange(ctx, obj.EnvironmentName, obj.TeamSlug, obj.WorkloadName, input.ResourceType, input.Start, input.End, input.Step())
}

func (r *workloadUtilizationDataResolver) Workload(ctx context.Context, obj *utilization.WorkloadUtilizationData) (workload.Workload, error) {
	return tryWorkload(ctx, obj.TeamSlug, obj.EnvironmentName, obj.WorkloadName)
}

func (r *Resolver) TeamServiceUtilization() gengqlv1.TeamServiceUtilizationResolver {
	return &teamServiceUtilizationResolver{r}
}

func (r *Resolver) TeamUtilizationData() gengqlv1.TeamUtilizationDataResolver {
	return &teamUtilizationDataResolver{r}
}

func (r *Resolver) TeamUtilizationEnvironmentDataPoint() gengqlv1.TeamUtilizationEnvironmentDataPointResolver {
	return &teamUtilizationEnvironmentDataPointResolver{r}
}

func (r *Resolver) WorkloadUtilization() gengqlv1.WorkloadUtilizationResolver {
	return &workloadUtilizationResolver{r}
}

func (r *Resolver) WorkloadUtilizationData() gengqlv1.WorkloadUtilizationDataResolver {
	return &workloadUtilizationDataResolver{r}
}

type (
	teamServiceUtilizationResolver              struct{ *Resolver }
	teamUtilizationDataResolver                 struct{ *Resolver }
	teamUtilizationEnvironmentDataPointResolver struct{ *Resolver }
	workloadUtilizationResolver                 struct{ *Resolver }
	workloadUtilizationDataResolver             struct{ *Resolver }
)
