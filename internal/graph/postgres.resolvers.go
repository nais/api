package graph

import (
	"context"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/persistence/postgres"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/team"
	"github.com/nais/api/internal/workload"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
)

func (r *applicationResolver) PostgresInstances(ctx context.Context, obj *application.Application, orderBy *postgres.PostgresInstanceOrder) (*pagination.FacetableConnection[*postgres.PostgresInstance, *postgres.PostgresInstanceFilter], error) {
	if obj.Spec == nil || obj.Spec.Uses == nil {
		return pagination.NewFacetableConnection(pagination.EmptyConnection[*postgres.PostgresInstance](), nil, (*postgres.PostgresInstanceFilter)(nil)), nil
	}
	instances, err := postgres.ListForWorkload(ctx, obj.TeamSlug, obj.EnvironmentName, obj.Spec.Uses.Postgres)
	if err != nil {
		return nil, err
	}
	return pagination.NewFacetableConnection(pagination.NewConnectionWithoutPagination(instances), instances, (*postgres.PostgresInstanceFilter)(nil)), nil
}

func (r *jobResolver) PostgresInstances(ctx context.Context, obj *job.Job, orderBy *postgres.PostgresInstanceOrder) (*pagination.FacetableConnection[*postgres.PostgresInstance, *postgres.PostgresInstanceFilter], error) {
	if obj.Spec == nil || obj.Spec.Uses == nil {
		return pagination.NewFacetableConnection(pagination.EmptyConnection[*postgres.PostgresInstance](), nil, (*postgres.PostgresInstanceFilter)(nil)), nil
	}
	instances, err := postgres.ListForWorkload(ctx, obj.TeamSlug, obj.EnvironmentName, obj.Spec.Uses.Postgres)
	if err != nil {
		return nil, err
	}
	return pagination.NewFacetableConnection(pagination.NewConnectionWithoutPagination(instances), instances, (*postgres.PostgresInstanceFilter)(nil)), nil
}

func (r *mutationResolver) CreatePostgresAccess(ctx context.Context, input postgres.CreatePostgresAccessInput) (*postgres.CreatePostgresAccessPayload, error) {
	if err := authz.CanGrantPostgresAccess(ctx, input.TeamSlug); err != nil {
		return nil, err
	}

	return postgres.CreatePostgresAccess(ctx, input)
}

func (r *mutationResolver) DeletePostgres(ctx context.Context, input postgres.DeletePostgresInput) (*postgres.DeletePostgresPayload, error) {
	if err := authz.CanDeletePostgres(ctx, input.TeamSlug); err != nil {
		return nil, err
	}
	return postgres.Delete(ctx, input)
}

func (r *postgresAccessResolver) Team(ctx context.Context, obj *postgres.PostgresAccess) (*team.Team, error) {
	return team.Get(ctx, obj.TeamSlug)
}

func (r *postgresAccessResolver) TeamEnvironment(ctx context.Context, obj *postgres.PostgresAccess) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
}

func (r *postgresAccessResolver) PostgresInstance(ctx context.Context, obj *postgres.PostgresAccess) (*postgres.PostgresInstance, error) {
	return postgres.GetPostgresInstance(ctx, obj.TeamSlug, obj.EnvironmentName, obj.PostgresInstanceName)
}

func (r *postgresInstanceResolver) Team(ctx context.Context, obj *postgres.PostgresInstance) (*team.Team, error) {
	return team.Get(ctx, obj.TeamSlug)
}

func (r *postgresInstanceResolver) TeamEnvironment(ctx context.Context, obj *postgres.PostgresInstance) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
}

func (r *postgresInstanceResolver) Postgres(ctx context.Context, obj *postgres.PostgresInstance) (*postgres.Postgres, error) {
	return postgres.GetPostgres(ctx, obj.TeamSlug, obj.EnvironmentName, obj.PostgresName)
}

func (r *postgresInstanceResolver) Workloads(ctx context.Context, obj *postgres.PostgresInstance, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor) (*pagination.Connection[workload.Workload], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	workloads := postgres.WorkloadsForInstance(ctx, obj.TeamSlug, obj.EnvironmentName, obj.Name)

	return pagination.NewConnection(pagination.Slice(workloads, page), page, len(workloads)), nil
}

func (r *postgresInstanceConnectionResolver) Facets(ctx context.Context, obj *pagination.FacetableConnection[*postgres.PostgresInstance, *postgres.PostgresInstanceFilter]) (*postgres.PostgresInstanceFacets, error) {
	return &postgres.PostgresInstanceFacets{
		AllInstances: obj.GetAllItems(),
		Filter:       obj.GetFilter(),
	}, nil
}

func (r *queryResolver) PostgresAccessConnection(ctx context.Context, input postgres.PostgresAccessConnectionInput) (*postgres.PostgresAccessConnection, error) {
	return postgres.GetPostgresAccessConnection(ctx, input)
}

func (r *queryResolver) PostgresAccess(ctx context.Context, name string, teamSlug slug.Slug, environmentName string) (*postgres.PostgresAccess, error) {
	return postgres.GetPostgresAccess(ctx, name, teamSlug, environmentName)
}

func (r *teamResolver) PostgresInstances(ctx context.Context, obj *team.Team, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *postgres.PostgresInstanceOrder, filter *postgres.PostgresInstanceFilter) (*pagination.FacetableConnection[*postgres.PostgresInstance, *postgres.PostgresInstanceFilter], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return postgres.ListForTeam(ctx, obj.Slug, page, orderBy, filter)
}

func (r *teamEnvironmentResolver) Postgres(ctx context.Context, obj *team.TeamEnvironment, name string) (*postgres.Postgres, error) {
	return postgres.GetPostgres(ctx, obj.TeamSlug, obj.EnvironmentName, name)
}

func (r *teamEnvironmentResolver) PostgresInstance(ctx context.Context, obj *team.TeamEnvironment, name string) (*postgres.PostgresInstance, error) {
	return postgres.GetPostgresInstance(ctx, obj.TeamSlug, obj.EnvironmentName, name)
}

func (r *teamInventoryCountsResolver) PostgresInstances(ctx context.Context, obj *team.TeamInventoryCounts) (*postgres.TeamInventoryCountPostgresInstances, error) {
	return &postgres.TeamInventoryCountPostgresInstances{
		Total: postgres.CountForTeam(ctx, obj.TeamSlug),
	}, nil
}

func (r *Resolver) PostgresAccess() gengql.PostgresAccessResolver { return &postgresAccessResolver{r} }

func (r *Resolver) PostgresInstance() gengql.PostgresInstanceResolver {
	return &postgresInstanceResolver{r}
}

func (r *Resolver) PostgresInstanceConnection() gengql.PostgresInstanceConnectionResolver {
	return &postgresInstanceConnectionResolver{r}
}

type (
	postgresAccessResolver             struct{ *Resolver }
	postgresInstanceResolver           struct{ *Resolver }
	postgresInstanceConnectionResolver struct{ *Resolver }
)
