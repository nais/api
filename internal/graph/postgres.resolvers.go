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

func (r *applicationResolver) Postgreses(ctx context.Context, obj *application.Application) (*pagination.FacetableConnection[*postgres.Postgres, *postgres.TeamPostgresFilter], error) {
	if obj.Spec == nil || obj.Spec.Uses == nil {
		return pagination.NewFacetableConnection(pagination.EmptyConnection[*postgres.Postgres](), nil, (*postgres.TeamPostgresFilter)(nil)), nil
	}
	instances, err := postgres.ListPostgresForWorkload(ctx, obj.TeamSlug, obj.EnvironmentName, obj.Spec.Uses.Postgres)
	if err != nil {
		return nil, err
	}
	return pagination.NewFacetableConnection(pagination.NewConnectionWithoutPagination(instances), instances, (*postgres.TeamPostgresFilter)(nil)), nil
}

func (r *jobResolver) Postgreses(ctx context.Context, obj *job.Job) (*pagination.FacetableConnection[*postgres.Postgres, *postgres.TeamPostgresFilter], error) {
	if obj.Spec == nil || obj.Spec.Uses == nil {
		return pagination.NewFacetableConnection(pagination.EmptyConnection[*postgres.Postgres](), nil, (*postgres.TeamPostgresFilter)(nil)), nil
	}
	instances, err := postgres.ListPostgresForWorkload(ctx, obj.TeamSlug, obj.EnvironmentName, obj.Spec.Uses.Postgres)
	if err != nil {
		return nil, err
	}
	return pagination.NewFacetableConnection(pagination.NewConnectionWithoutPagination(instances), instances, (*postgres.TeamPostgresFilter)(nil)), nil
}

func (r *mutationResolver) CreatePostgresAccess(ctx context.Context, input postgres.CreatePostgresAccessInput) (*postgres.CreatePostgresAccessPayload, error) {
	if err := authz.CanGrantPostgresAccess(ctx, input.TeamSlug); err != nil {
		return nil, err
	}

	return postgres.CreatePostgresAccess(ctx, input)
}

func (r *mutationResolver) CreatePostgres(ctx context.Context, input postgres.CreatePostgresInput) (*postgres.CreatePostgresPayload, error) {
	if err := authz.CanCreatePostgres(ctx, input.TeamSlug); err != nil {
		return nil, err
	}
	return postgres.Create(ctx, input)
}

func (r *mutationResolver) UpdatePostgres(ctx context.Context, input postgres.UpdatePostgresInput) (*postgres.UpdatePostgresPayload, error) {
	if err := authz.CanUpdatePostgres(ctx, input.TeamSlug); err != nil {
		return nil, err
	}
	return postgres.Update(ctx, input)
}

func (r *mutationResolver) CreatePostgresBranch(ctx context.Context, input postgres.CreatePostgresBranchInput) (*postgres.CreatePostgresBranchPayload, error) {
	if err := authz.CanCreatePostgres(ctx, input.TeamSlug); err != nil {
		return nil, err
	}
	return postgres.CreateBranch(ctx, input)
}

func (r *mutationResolver) ActivatePostgresBranch(ctx context.Context, input postgres.ActivatePostgresBranchInput) (*postgres.ActivatePostgresBranchPayload, error) {
	if err := authz.CanUpdatePostgres(ctx, input.TeamSlug); err != nil {
		return nil, err
	}
	return postgres.ActivateBranch(ctx, input)
}

func (r *mutationResolver) DeletePostgresBranch(ctx context.Context, input postgres.DeletePostgresBranchInput) (*postgres.DeletePostgresBranchPayload, error) {
	if err := authz.CanDeletePostgres(ctx, input.TeamSlug); err != nil {
		return nil, err
	}
	return postgres.Delete(ctx, input)
}

func (r *mutationResolver) DeletePostgres(ctx context.Context, input postgres.DeletePostgresInput) (*postgres.DeletePostgresPayload, error) {
	if err := authz.CanDeletePostgres(ctx, input.TeamSlug); err != nil {
		return nil, err
	}
	return postgres.DeletePostgres(ctx, input)
}

func (r *postgresResolver) Team(ctx context.Context, obj *postgres.Postgres) (*team.Team, error) {
	return team.Get(ctx, obj.TeamSlug)
}

func (r *postgresResolver) TeamEnvironment(ctx context.Context, obj *postgres.Postgres) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
}

func (r *postgresResolver) ActiveBranch(ctx context.Context, obj *postgres.Postgres) (*postgres.PostgresBranch, error) {
	if obj.ActiveBranch == nil {
		return nil, nil
	}
	return postgres.GetPostgresBranch(ctx, obj.TeamSlug, obj.EnvironmentName, obj.Name, *obj.ActiveBranch)
}

func (r *postgresResolver) Branch(ctx context.Context, obj *postgres.Postgres, name string) (*postgres.PostgresBranch, error) {
	return postgres.GetPostgresBranch(ctx, obj.TeamSlug, obj.EnvironmentName, obj.Name, name)
}

func (r *postgresResolver) Branches(ctx context.Context, obj *postgres.Postgres, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *postgres.PostgresBranchOrder) (*pagination.FacetableConnection[*postgres.PostgresBranch, *postgres.PostgresBranchFilter], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}
	return postgres.ListForPostgres(ctx, obj, page, orderBy), nil
}

func (r *postgresAccessResolver) Team(ctx context.Context, obj *postgres.PostgresAccess) (*team.Team, error) {
	return team.Get(ctx, obj.TeamSlug)
}

func (r *postgresAccessResolver) TeamEnvironment(ctx context.Context, obj *postgres.PostgresAccess) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
}

func (r *postgresAccessResolver) PostgresBranch(ctx context.Context, obj *postgres.PostgresAccess) (*postgres.PostgresBranch, error) {
	return postgres.GetPostgresBranchByObjectName(ctx, obj.TeamSlug, obj.EnvironmentName, obj.PostgresBranchName)
}

func (r *postgresAccessResolver) Connection(ctx context.Context, obj *postgres.PostgresAccess) (*postgres.PostgresAccessConnectionDetails, error) {
	return postgres.GetPostgresAccessConnection(ctx, postgres.PostgresAccessConnectionInput{
		Name: obj.Name, TeamSlug: obj.TeamSlug, EnvironmentName: obj.EnvironmentName,
	})
}

func (r *postgresBranchResolver) Team(ctx context.Context, obj *postgres.PostgresBranch) (*team.Team, error) {
	return team.Get(ctx, obj.TeamSlug)
}

func (r *postgresBranchResolver) TeamEnvironment(ctx context.Context, obj *postgres.PostgresBranch) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
}

func (r *postgresBranchResolver) Postgres(ctx context.Context, obj *postgres.PostgresBranch) (*postgres.Postgres, error) {
	return postgres.GetPostgres(ctx, obj.TeamSlug, obj.EnvironmentName, obj.PostgresName)
}

func (r *postgresBranchResolver) Workloads(ctx context.Context, obj *postgres.PostgresBranch, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor) (*pagination.Connection[workload.Workload], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	workloads := postgres.WorkloadsForInstance(ctx, obj.TeamSlug, obj.EnvironmentName, obj.PostgresName, obj.Name)

	return pagination.NewConnection(pagination.Slice(workloads, page), page, len(workloads)), nil
}

func (r *postgresBranchConnectionResolver) Facets(ctx context.Context, obj *pagination.FacetableConnection[*postgres.PostgresBranch, *postgres.PostgresBranchFilter]) (*postgres.PostgresBranchFacets, error) {
	return &postgres.PostgresBranchFacets{
		AllInstances: obj.GetAllItems(),
		Filter:       obj.GetFilter(),
	}, nil
}

func (r *postgresConnectionResolver) Facets(ctx context.Context, obj *pagination.FacetableConnection[*postgres.Postgres, *postgres.TeamPostgresFilter]) (*postgres.PostgresFacets, error) {
	filtered := make([]*postgres.Postgres, 0, len(obj.GetAllItems()))
	for _, pg := range obj.GetAllItems() {
		if obj.GetFilter().Matches(pg) {
			filtered = append(filtered, pg)
		}
	}
	return &postgres.PostgresFacets{AllInstances: obj.GetAllItems(), FilteredInstances: filtered}, nil
}

func (r *teamResolver) Postgreses(ctx context.Context, obj *team.Team, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, filter *postgres.TeamPostgresFilter) (*pagination.FacetableConnection[*postgres.Postgres, *postgres.TeamPostgresFilter], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}
	return postgres.ListPostgresForTeam(ctx, obj.Slug, page, filter), nil
}

func (r *teamEnvironmentResolver) Postgres(ctx context.Context, obj *team.TeamEnvironment, name string) (*postgres.Postgres, error) {
	return postgres.GetPostgres(ctx, obj.TeamSlug, obj.EnvironmentName, name)
}

func (r *teamEnvironmentResolver) PostgresAccess(ctx context.Context, obj *team.TeamEnvironment, name string) (*postgres.PostgresAccess, error) {
	return postgres.GetPostgresAccess(ctx, name, obj.TeamSlug, obj.EnvironmentName)
}

func (r *teamInventoryCountsResolver) PostgresBranches(ctx context.Context, obj *team.TeamInventoryCounts) (*postgres.TeamInventoryCountPostgresBranches, error) {
	return &postgres.TeamInventoryCountPostgresBranches{
		Total: postgres.CountForTeam(ctx, obj.TeamSlug),
	}, nil
}

func (r *Resolver) Postgres() gengql.PostgresResolver { return &postgresResolver{r} }

func (r *Resolver) PostgresAccess() gengql.PostgresAccessResolver { return &postgresAccessResolver{r} }

func (r *Resolver) PostgresBranch() gengql.PostgresBranchResolver { return &postgresBranchResolver{r} }

func (r *Resolver) PostgresBranchConnection() gengql.PostgresBranchConnectionResolver {
	return &postgresBranchConnectionResolver{r}
}

func (r *Resolver) PostgresConnection() gengql.PostgresConnectionResolver {
	return &postgresConnectionResolver{r}
}

type (
	postgresResolver                 struct{ *Resolver }
	postgresAccessResolver           struct{ *Resolver }
	postgresBranchResolver           struct{ *Resolver }
	postgresBranchConnectionResolver struct{ *Resolver }
	postgresConnectionResolver       struct{ *Resolver }
)
