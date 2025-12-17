package graph

import (
	"context"
	"errors"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/team"
	"github.com/nais/api/internal/unleash"
)

func (r *mutationResolver) CreateUnleashForTeam(ctx context.Context, input unleash.CreateUnleashForTeamInput) (*unleash.CreateUnleashForTeamPayload, error) {
	if err := authz.CanCreateUnleash(ctx, input.TeamSlug); err != nil {
		return nil, err
	}

	instance, err := unleash.Create(ctx, &input)
	if err != nil {
		return nil, err
	}

	return &unleash.CreateUnleashForTeamPayload{Unleash: instance}, nil
}

func (r *mutationResolver) UpdateUnleashInstance(ctx context.Context, input unleash.UpdateUnleashInstanceInput) (*unleash.UpdateUnleashInstancePayload, error) {
	if err := authz.CanUpdateUnleash(ctx, input.TeamSlug); err != nil {
		return nil, err
	}

	instance, err := unleash.UpdateInstance(ctx, &input)
	if err != nil {
		return nil, err
	}

	return &unleash.UpdateUnleashInstancePayload{Unleash: instance}, nil
}

func (r *mutationResolver) AllowTeamAccessToUnleash(ctx context.Context, input unleash.AllowTeamAccessToUnleashInput) (*unleash.AllowTeamAccessToUnleashPayload, error) {
	if err := authz.CanUpdateUnleash(ctx, input.TeamSlug); err != nil {
		return nil, err
	}

	instance, err := unleash.AllowTeamAccess(ctx, input)
	if err != nil {
		return nil, err
	}

	return &unleash.AllowTeamAccessToUnleashPayload{Unleash: instance}, nil
}

func (r *mutationResolver) RevokeTeamAccessToUnleash(ctx context.Context, input unleash.RevokeTeamAccessToUnleashInput) (*unleash.RevokeTeamAccessToUnleashPayload, error) {
	if err := authz.CanUpdateUnleash(ctx, input.TeamSlug); err != nil {
		return nil, err
	}

	instance, err := unleash.RevokeTeamAccess(ctx, input)
	if err != nil {
		return nil, err
	}

	return &unleash.RevokeTeamAccessToUnleashPayload{Unleash: instance}, nil
}

func (r *mutationResolver) DeleteUnleashInstance(ctx context.Context, input unleash.DeleteUnleashInstanceInput) (*unleash.DeleteUnleashInstancePayload, error) {
	if err := authz.CanUpdateUnleash(ctx, input.TeamSlug); err != nil {
		return nil, err
	}

	payload, err := unleash.Delete(ctx, &input)
	if err != nil {
		return nil, err
	}

	return payload, nil
}

func (r *queryResolver) UnleashReleaseChannels(ctx context.Context) ([]*unleash.UnleashReleaseChannel, error) {
	return unleash.GetReleaseChannels(ctx)
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

func (r *Resolver) UnleashInstance() gengql.UnleashInstanceResolver {
	return &unleashInstanceResolver{r}
}

func (r *Resolver) UnleashInstanceMetrics() gengql.UnleashInstanceMetricsResolver {
	return &unleashInstanceMetricsResolver{r}
}

type (
	unleashInstanceResolver        struct{ *Resolver }
	unleashInstanceMetricsResolver struct{ *Resolver }
)
