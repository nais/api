package graphv1

import (
	"context"
	"errors"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/v1/graphv1/gengqlv1"
	"github.com/nais/api/internal/v1/graphv1/pagination"
	"github.com/nais/api/internal/v1/kubernetes/watcher"
	"github.com/nais/api/internal/v1/role"
	"github.com/nais/api/internal/v1/team"
	"github.com/nais/api/internal/v1/unleash"
)

func (r *mutationResolver) CreateUnleashForTeam(ctx context.Context, input unleash.CreateUnleashInstanceInput) (*unleash.CreateUnleashInstancePayload, error) {
	if err := authz.RequireTeamAuthorizationCtx(ctx, role.AuthorizationUnleashCreate, input.TeamSlug); err != nil {
		return nil, err
	}

	instance, err := unleash.Create(ctx, &input)
	if err != nil {
		return nil, err
	}

	return &unleash.CreateUnleashInstancePayload{Unleash: instance}, nil
}

func (r *mutationResolver) AllowTeamAccessToUnleash(ctx context.Context, input unleash.AllowTeamAccessToUnleashInput) (*unleash.AllowTeamAccessToUnleashPayload, error) {
	if err := authz.RequireTeamAuthorizationCtx(ctx, role.AuthorizationUnleashUpdate, input.TeamSlug); err != nil {
		return nil, err
	}

	instance, err := unleash.AllowTeamAccess(ctx, input)
	if err != nil {
		return nil, err
	}

	return &unleash.AllowTeamAccessToUnleashPayload{Unleash: instance}, nil
}

func (r *mutationResolver) RevokeTeamAccessToUnleash(ctx context.Context, input unleash.RevokeTeamAccessToUnleashInput) (*unleash.RevokeTeamAccessToUnleashPayload, error) {
	if err := authz.RequireTeamAuthorizationCtx(ctx, role.AuthorizationUnleashUpdate, input.TeamSlug); err != nil {
		return nil, err
	}

	instance, err := unleash.RevokeTeamAccess(ctx, input)
	if err != nil {
		return nil, err
	}

	return &unleash.RevokeTeamAccessToUnleashPayload{Unleash: instance}, nil
}

func (r *teamResolver) Unleash(ctx context.Context, obj *team.Team) (*unleash.UnleashInstance, error) {
	ins, err := unleash.ForTeam(ctx, obj.Slug)
	if err != nil && !errors.Is(err, &watcher.ErrorNotFound{}) {
		return nil, err
	}
	return ins, nil
}

func (r *unleashInstanceResolver) AllowedTeams(ctx context.Context, obj *unleash.UnleashInstance, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor) (*pagination.Connection[*team.Team], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return team.ListBySlugs(ctx, obj.AllowedTeamSlugs, page)
}

func (r *unleashInstanceMetricsResolver) Toggles(ctx context.Context, obj *unleash.UnleashInstanceMetrics) (int, error) {
	return unleash.Toggles(ctx, obj.TeamSlug)
}

func (r *unleashInstanceMetricsResolver) APITokens(ctx context.Context, obj *unleash.UnleashInstanceMetrics) (int, error) {
	return unleash.APITokens(ctx, obj.TeamSlug)
}

func (r *unleashInstanceMetricsResolver) CPUUtilization(ctx context.Context, obj *unleash.UnleashInstanceMetrics) (float64, error) {
	usage, err := unleash.CPUUsage(ctx, obj.TeamSlug)
	if err != nil {
		return 0, err
	}

	return usage / obj.CPURequests * 100, nil
}

func (r *unleashInstanceMetricsResolver) MemoryUtilization(ctx context.Context, obj *unleash.UnleashInstanceMetrics) (float64, error) {
	usage, err := unleash.MemoryUsage(ctx, obj.TeamSlug)
	if err != nil {
		return 0, err
	}

	return usage / obj.MemoryRequests * 100, nil
}

func (r *Resolver) UnleashInstance() gengqlv1.UnleashInstanceResolver {
	return &unleashInstanceResolver{r}
}

func (r *Resolver) UnleashInstanceMetrics() gengqlv1.UnleashInstanceMetricsResolver {
	return &unleashInstanceMetricsResolver{r}
}

type (
	unleashInstanceResolver        struct{ *Resolver }
	unleashInstanceMetricsResolver struct{ *Resolver }
)