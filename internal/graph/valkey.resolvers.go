package graph

import (
	"context"

	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/persistence/valkey"
	"github.com/nais/api/internal/team"
	"github.com/nais/api/internal/workload"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
)

func (r *applicationResolver) ValkeyInstances(ctx context.Context, obj *application.Application, orderBy *valkey.ValkeyOrder) (*pagination.Connection[*valkey.Valkey], error) {
	return valkey.ListForWorkload(ctx, obj.TeamSlug, obj.GetEnvironmentName(), obj.Spec.Valkey, orderBy)
}

func (r *jobResolver) ValkeyInstances(ctx context.Context, obj *job.Job, orderBy *valkey.ValkeyOrder) (*pagination.Connection[*valkey.Valkey], error) {
	return valkey.ListForWorkload(ctx, obj.TeamSlug, obj.GetEnvironmentName(), obj.Spec.Valkey, orderBy)
}

func (r *teamResolver) ValkeyInstances(ctx context.Context, obj *team.Team, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *valkey.ValkeyOrder) (*pagination.Connection[*valkey.Valkey], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return valkey.ListForTeam(ctx, obj.Slug, page, orderBy)
}

func (r *teamEnvironmentResolver) ValkeyInstance(ctx context.Context, obj *team.TeamEnvironment, name string) (*valkey.Valkey, error) {
	return valkey.Get(ctx, obj.TeamSlug, obj.EnvironmentName, name)
}

func (r *teamInventoryCountsResolver) ValkeyInstances(ctx context.Context, obj *team.TeamInventoryCounts) (*valkey.TeamInventoryCountValkeys, error) {
	return &valkey.TeamInventoryCountValkeys{
		Total: len(valkey.ListAllForTeam(ctx, obj.TeamSlug)),
	}, nil
}

func (r *valkeyInstanceResolver) Team(ctx context.Context, obj *valkey.Valkey) (*team.Team, error) {
	return team.Get(ctx, obj.TeamSlug)
}

func (r *valkeyInstanceResolver) Environment(ctx context.Context, obj *valkey.Valkey) (*team.TeamEnvironment, error) {
	return r.TeamEnvironment(ctx, obj)
}

func (r *valkeyInstanceResolver) TeamEnvironment(ctx context.Context, obj *valkey.Valkey) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
}

func (r *valkeyInstanceResolver) Access(ctx context.Context, obj *valkey.Valkey, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *valkey.ValkeyAccessOrder) (*pagination.Connection[*valkey.ValkeyAccess], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return valkey.ListAccess(ctx, obj, page, orderBy)
}

func (r *valkeyInstanceResolver) Workload(ctx context.Context, obj *valkey.Valkey) (workload.Workload, error) {
	return getWorkload(ctx, obj.WorkloadReference, obj.TeamSlug, obj.EnvironmentName)
}

func (r *valkeyInstanceAccessResolver) Workload(ctx context.Context, obj *valkey.ValkeyAccess) (workload.Workload, error) {
	return getWorkload(ctx, obj.WorkloadReference, obj.TeamSlug, obj.EnvironmentName)
}

func (r *Resolver) ValkeyInstance() gengql.ValkeyInstanceResolver { return &valkeyInstanceResolver{r} }

func (r *Resolver) ValkeyInstanceAccess() gengql.ValkeyInstanceAccessResolver {
	return &valkeyInstanceAccessResolver{r}
}

type (
	valkeyInstanceResolver       struct{ *Resolver }
	valkeyInstanceAccessResolver struct{ *Resolver }
)
