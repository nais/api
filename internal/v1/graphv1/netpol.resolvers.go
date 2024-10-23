package graphv1

import (
	"context"
	"errors"

	"github.com/nais/api/internal/v1/graphv1/gengqlv1"
	"github.com/nais/api/internal/v1/kubernetes/watcher"
	"github.com/nais/api/internal/v1/team"
	"github.com/nais/api/internal/v1/workload"
	"github.com/nais/api/internal/v1/workload/application"
	"github.com/nais/api/internal/v1/workload/job"
	"github.com/nais/api/internal/v1/workload/netpol"
)

func (r *applicationResolver) NetworkPolicy(ctx context.Context, obj *application.Application) (*netpol.NetworkPolicy, error) {
	return netpol.ListForWorkload(ctx, obj.TeamSlug, obj.EnvironmentName, obj.Name, obj.Spec.AccessPolicy), nil
}

func (r *jobResolver) NetworkPolicy(ctx context.Context, obj *job.Job) (*netpol.NetworkPolicy, error) {
	return netpol.ListForWorkload(ctx, obj.TeamSlug, obj.EnvironmentName, obj.Name, obj.Spec.AccessPolicy), nil
}

func (r *networkPolicyRuleResolver) TargetWorkload(ctx context.Context, obj *netpol.NetworkPolicyRule) (workload.Workload, error) {
	w, err := tryWorkload(ctx, obj.TargetTeamSlug, obj.EnvironmentName, obj.TargetWorkloadName)
	if errors.Is(err, &watcher.ErrorNotFound{}) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	return w, nil
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
