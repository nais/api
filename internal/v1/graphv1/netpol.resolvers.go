package graphv1

import (
	"context"

	"github.com/nais/api/internal/v1/graphv1/gengqlv1"
	"github.com/nais/api/internal/v1/team"
	"github.com/nais/api/internal/v1/workload"
	"github.com/nais/api/internal/v1/workload/application"
	"github.com/nais/api/internal/v1/workload/job"
	"github.com/nais/api/internal/v1/workload/netpol"
)

func (r *applicationResolver) NetworkPolicy(ctx context.Context, obj *application.Application) (*netpol.NetworkPolicy, error) {
	return netpol.ListForWorkload(ctx, obj.TeamSlug, obj.EnvironmentName, obj.Name, obj.Spec.AccessPolicy)
}

func (r *jobResolver) NetworkPolicy(ctx context.Context, obj *job.Job) (*netpol.NetworkPolicy, error) {
	return netpol.ListForWorkload(ctx, obj.TeamSlug, obj.EnvironmentName, obj.Name, obj.Spec.AccessPolicy)
}

func (r *networkPolicyRuleResolver) TargetWorkload(ctx context.Context, obj *netpol.NetworkPolicyRule) (workload.Workload, error) {
	return tryWorkload(ctx, obj.TargetTeamSlug, obj.EnvironmentName, obj.TargetWorkloadName)
}

func (r *networkPolicyRuleResolver) TargetTeam(ctx context.Context, obj *netpol.NetworkPolicyRule) (*team.Team, error) {
	return team.Get(ctx, obj.TargetTeamSlug)
}

func (r *networkPolicyRuleResolver) Mutual(ctx context.Context, obj *netpol.NetworkPolicyRule) (bool, error) {
	if obj.IsOutbound {
		return netpol.AllowsInboundWorkload(ctx, obj.TargetTeamSlug, obj.EnvironmentName, obj.TargetWorkloadName, obj.TeamSlug, obj.WorkloadName), nil
	}

	return netpol.AllowsOutboundWorkload(ctx, obj.TargetTeamSlug, obj.EnvironmentName, obj.TargetWorkloadName, obj.TeamSlug, obj.WorkloadName), nil
}

func (r *Resolver) NetworkPolicyRule() gengqlv1.NetworkPolicyRuleResolver {
	return &networkPolicyRuleResolver{r}
}

type networkPolicyRuleResolver struct{ *Resolver }