package graph

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/slug"
)

func (r *mutationResolver) CreateUnleashForTeam(ctx context.Context, team slug.Slug) (*model.Unleash, error) {
	actor := authz.ActorFromContext(ctx)
	err := authz.RequireTeamMembership(actor, team)
	if err != nil {
		return nil, err
	}

	unleashName := team.String()

	ret, err := r.unleashMgr.NewUnleash(ctx, unleashName, []string{team.String()})
	if err != nil {
		return nil, err
	}

	err = r.auditor.UnleashCreated(ctx, actor.User, team, unleashName)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

func (r *mutationResolver) UpdateUnleashForTeam(ctx context.Context, team slug.Slug, name string, allowedTeams []string) (*model.Unleash, error) {
	actor := authz.ActorFromContext(ctx)
	err := authz.RequireTeamMembership(actor, team)
	if err != nil {
		return nil, err
	}

	if len(allowedTeams) == 0 {
		return nil, apierror.ErrUnleashEmptyAllowedTeams
	}

	ret, err := r.unleashMgr.UpdateUnleash(ctx, name, allowedTeams)
	if err != nil {
		return nil, err
	}

	// TODO: split mutation (e.g. AddAllowedTeam, RemoveAllowedTeam) to allow for more granular auditing?
	err = r.auditor.UnleashUpdated(ctx, actor.User, team, name)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

func (r *unleashMetricsResolver) Toggles(ctx context.Context, obj *model.UnleashMetrics) (int, error) {
	if obj.GQLVars.InstanceName == "" {
		r.log.Debugf("InstanceName is empty, skipping toggles query")
		return 0, nil
	}
	toggles, err := r.unleashMgr.PromQuery(ctx, fmt.Sprintf("sum(feature_toggles_total{job=~\"%s\", namespace=\"%s\"})", obj.GQLVars.InstanceName, obj.GQLVars.Namespace))
	if err != nil {
		return 0, err
	}
	return int(toggles), nil
}

func (r *unleashMetricsResolver) APITokens(ctx context.Context, obj *model.UnleashMetrics) (int, error) {
	if obj.GQLVars.InstanceName == "" {
		r.log.Debugf("InstanceName is empty, skipping APITokens query")
		return 0, nil
	}
	apiTokens, err := r.unleashMgr.PromQuery(ctx, fmt.Sprintf("sum(client_apps_total{job=~\"%s\", namespace=\"%s\", range=\"allTime\"})", obj.GQLVars.InstanceName, obj.GQLVars.Namespace))
	if err != nil {
		return 0, err
	}
	return int(apiTokens), nil
}

func (r *unleashMetricsResolver) CPUUtilization(ctx context.Context, obj *model.UnleashMetrics) (float64, error) {
	if obj.GQLVars.InstanceName == "" {
		r.log.Debugf("InstanceName is empty, skipping CPU utilization query")
		return 0, nil
	}

	cpu, err := r.unleashMgr.PromQuery(ctx, fmt.Sprintf("irate(process_cpu_user_seconds_total{job=\"%s\", namespace=\"%s\"}[2m])", obj.GQLVars.InstanceName, obj.GQLVars.Namespace))
	if err != nil || cpu == 0 || obj.CpuRequests == 0 {
		return 0, err
	}
	return float64(cpu) / obj.CpuRequests * 100, nil
}

func (r *unleashMetricsResolver) MemoryUtilization(ctx context.Context, obj *model.UnleashMetrics) (float64, error) {
	if obj.GQLVars.InstanceName == "" {
		r.log.Debugf("InstanceName is empty, skipping memory utilization query")
		return 0, nil
	}
	memory, err := r.unleashMgr.PromQuery(ctx, fmt.Sprintf("process_resident_memory_bytes{job=\"%s\", namespace=\"%s\"}", obj.GQLVars.InstanceName, obj.GQLVars.Namespace))
	if err != nil || memory == 0 || obj.MemoryRequests == 0 {
		return 0, err
	}
	return float64(memory) / obj.MemoryRequests * 100, nil
}

func (r *Resolver) UnleashMetrics() gengql.UnleashMetricsResolver { return &unleashMetricsResolver{r} }

type unleashMetricsResolver struct{ *Resolver }
