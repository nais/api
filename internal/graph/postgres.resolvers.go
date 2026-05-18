package graph

import (
	"context"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/persistence/postgres"
	"github.com/nais/api/internal/team"
	"github.com/nais/api/internal/workload"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
)

func (r *applicationResolver) PostgresInstances(ctx context.Context, obj *application.Application, orderBy *postgres.PostgresInstanceOrder) (*postgres.PostgresInstanceConnection, error) {
	if obj.Spec.Postgres == nil || obj.Spec.Postgres.ClusterName == "" {
		return postgres.NewPostgresInstanceConnection(pagination.EmptyConnection[*postgres.PostgresInstance](), nil, nil), nil
	}

	instance, err := postgres.GetForWorkload(ctx, obj.TeamSlug, obj.EnvironmentName, obj.Spec.Postgres.ClusterName)
	if err != nil {
		return nil, err
	}

	if instance == nil {
		return postgres.NewPostgresInstanceConnection(pagination.EmptyConnection[*postgres.PostgresInstance](), nil, nil), nil
	}

	instances := []*postgres.PostgresInstance{instance}
	return postgres.NewPostgresInstanceConnection(pagination.NewConnectionWithoutPagination(instances), instances, nil), nil
}

func (r *jobResolver) PostgresInstances(ctx context.Context, obj *job.Job, orderBy *postgres.PostgresInstanceOrder) (*postgres.PostgresInstanceConnection, error) {
	if obj.Spec.Postgres == nil || obj.Spec.Postgres.ClusterName == "" {
		return postgres.NewPostgresInstanceConnection(pagination.EmptyConnection[*postgres.PostgresInstance](), nil, nil), nil
	}

	instance, err := postgres.GetForWorkload(ctx, obj.TeamSlug, obj.EnvironmentName, obj.Spec.Postgres.ClusterName)
	if err != nil {
		return nil, err
	}

	if instance == nil {
		return postgres.NewPostgresInstanceConnection(pagination.EmptyConnection[*postgres.PostgresInstance](), nil, nil), nil
	}

	instances := []*postgres.PostgresInstance{instance}
	return postgres.NewPostgresInstanceConnection(pagination.NewConnectionWithoutPagination(instances), instances, nil), nil
}

func (r *mutationResolver) GrantPostgresAccess(ctx context.Context, input postgres.GrantPostgresAccessInput) (*postgres.GrantPostgresAccessPayload, error) {
	if err := authz.CanGrantPostgresAccess(ctx, input.TeamSlug); err != nil {
		return nil, err
	}

	if err := postgres.GrantZalandoPostgresAccess(ctx, input); err != nil {
		return nil, err
	}

	return &postgres.GrantPostgresAccessPayload{
		Error: new(string),
	}, nil
}

func (r *mutationResolver) DeletePostgres(ctx context.Context, input postgres.DeletePostgresInput) (*postgres.DeletePostgresPayload, error) {
	if err := authz.CanDeletePostgres(ctx, input.TeamSlug); err != nil {
		return nil, err
	}
	return postgres.Delete(ctx, input)
}

func (r *postgresInstanceResolver) Team(ctx context.Context, obj *postgres.PostgresInstance) (*team.Team, error) {
	return team.Get(ctx, obj.TeamSlug)
}

func (r *postgresInstanceResolver) Environment(ctx context.Context, obj *postgres.PostgresInstance) (*team.TeamEnvironment, error) {
	return r.TeamEnvironment(ctx, obj)
}

func (r *postgresInstanceResolver) TeamEnvironment(ctx context.Context, obj *postgres.PostgresInstance) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
}

func (r *postgresInstanceResolver) Workloads(ctx context.Context, obj *postgres.PostgresInstance, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor) (*pagination.Connection[workload.Workload], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	workloads := postgres.WorkloadsForInstance(ctx, obj.TeamSlug, obj.EnvironmentName, obj.Name)

	return pagination.NewConnection(pagination.Slice(workloads, page), page, len(workloads)), nil
}

func (r *postgresInstanceAuditResolver) URL(ctx context.Context, obj *postgres.PostgresInstanceAudit) (*string, error) {
	return postgres.GetAuditURL(ctx, obj)
}

func (r *postgresInstanceConnectionResolver) Facets(ctx context.Context, obj *postgres.PostgresInstanceConnection) (*postgres.PostgresInstanceFacets, error) {
	return postgres.ComputeFacets(obj.GetAllInstances(), obj.GetFilter()), nil
}

func (r *teamResolver) PostgresInstances(ctx context.Context, obj *team.Team, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *postgres.PostgresInstanceOrder, filter *postgres.PostgresInstanceFilter) (*postgres.PostgresInstanceConnection, error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return postgres.ListForTeam(ctx, obj.Slug, page, orderBy, filter)
}

func (r *teamEnvironmentResolver) PostgresInstance(ctx context.Context, obj *team.TeamEnvironment, name string) (*postgres.PostgresInstance, error) {
	return postgres.GetZalandoPostgres(ctx, obj.TeamSlug, obj.EnvironmentName, name)
}

func (r *teamInventoryCountsResolver) PostgresInstances(ctx context.Context, obj *team.TeamInventoryCounts) (*postgres.TeamInventoryCountPostgresInstances, error) {
	return &postgres.TeamInventoryCountPostgresInstances{
		Total: postgres.CountForTeam(ctx, obj.TeamSlug),
	}, nil
}

func (r *Resolver) PostgresInstance() gengql.PostgresInstanceResolver {
	return &postgresInstanceResolver{r}
}

func (r *Resolver) PostgresInstanceAudit() gengql.PostgresInstanceAuditResolver {
	return &postgresInstanceAuditResolver{r}
}

func (r *Resolver) PostgresInstanceConnection() gengql.PostgresInstanceConnectionResolver {
	return &postgresInstanceConnectionResolver{r}
}

type (
	postgresInstanceResolver           struct{ *Resolver }
	postgresInstanceAuditResolver      struct{ *Resolver }
	postgresInstanceConnectionResolver struct{ *Resolver }
)
