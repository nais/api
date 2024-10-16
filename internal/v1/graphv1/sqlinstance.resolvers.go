package graphv1

import (
	"context"
	"errors"

	"github.com/nais/api/internal/v1/graphv1/gengqlv1"
	"github.com/nais/api/internal/v1/graphv1/pagination"
	"github.com/nais/api/internal/v1/kubernetes/watcher"
	"github.com/nais/api/internal/v1/persistence/sqlinstance"
	"github.com/nais/api/internal/v1/team"
	"github.com/nais/api/internal/v1/workload"
	"github.com/nais/api/internal/v1/workload/application"
	"github.com/nais/api/internal/v1/workload/job"
)

func (r *applicationResolver) SQLInstances(ctx context.Context, obj *application.Application, orderBy *sqlinstance.SQLInstanceOrder) (*pagination.Connection[*sqlinstance.SQLInstance], error) {
	if obj.Spec.GCP == nil || len(obj.Spec.GCP.SqlInstances) == 0 {
		return pagination.EmptyConnection[*sqlinstance.SQLInstance](), nil
	}

	return sqlinstance.ListForWorkload(ctx, obj.TeamSlug, obj.EnvironmentName, obj.Spec.GCP.SqlInstances, orderBy)
}

func (r *jobResolver) SQLInstances(ctx context.Context, obj *job.Job, orderBy *sqlinstance.SQLInstanceOrder) (*pagination.Connection[*sqlinstance.SQLInstance], error) {
	if obj.Spec.GCP == nil || len(obj.Spec.GCP.SqlInstances) == 0 {
		return pagination.EmptyConnection[*sqlinstance.SQLInstance](), nil
	}

	return sqlinstance.ListForWorkload(ctx, obj.TeamSlug, obj.EnvironmentName, obj.Spec.GCP.SqlInstances, orderBy)
}

func (r *sqlDatabaseResolver) Team(ctx context.Context, obj *sqlinstance.SQLDatabase) (*team.Team, error) {
	return team.Get(ctx, obj.TeamSlug)
}

func (r *sqlDatabaseResolver) Environment(ctx context.Context, obj *sqlinstance.SQLDatabase) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
}

func (r *sqlInstanceResolver) Team(ctx context.Context, obj *sqlinstance.SQLInstance) (*team.Team, error) {
	return team.Get(ctx, obj.TeamSlug)
}

func (r *sqlInstanceResolver) Environment(ctx context.Context, obj *sqlinstance.SQLInstance) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
}

func (r *sqlInstanceResolver) Workload(ctx context.Context, obj *sqlinstance.SQLInstance) (workload.Workload, error) {
	return getWorkload(ctx, obj.WorkloadReference, obj.TeamSlug, obj.EnvironmentName)
}

func (r *sqlInstanceResolver) Database(ctx context.Context, obj *sqlinstance.SQLInstance) (*sqlinstance.SQLDatabase, error) {
	db, err := sqlinstance.GetDatabase(ctx, obj.TeamSlug, obj.EnvironmentName, obj.Name)
	if err != nil {
		var errNotFound *watcher.ErrorNotFound

		if errors.As(err, &errNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return db, err
}

func (r *sqlInstanceResolver) Flags(ctx context.Context, obj *sqlinstance.SQLInstance, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor) (*pagination.Connection[*sqlinstance.SQLInstanceFlag], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	ret := pagination.Slice(obj.Flags, page)
	return pagination.NewConnection(ret, page, int32(len(obj.Flags))), nil
}

func (r *sqlInstanceResolver) Users(ctx context.Context, obj *sqlinstance.SQLInstance, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *sqlinstance.SQLInstanceUserOrder) (*pagination.Connection[*sqlinstance.SQLInstanceUser], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return sqlinstance.ListSQLInstanceUsers(ctx, obj, page, orderBy)
}

func (r *teamResolver) SQLInstances(ctx context.Context, obj *team.Team, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *sqlinstance.SQLInstanceOrder) (*pagination.Connection[*sqlinstance.SQLInstance], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return sqlinstance.ListForTeam(ctx, obj.Slug, page, orderBy)
}

func (r *teamEnvironmentResolver) SQLInstance(ctx context.Context, obj *team.TeamEnvironment, name string) (*sqlinstance.SQLInstance, error) {
	return sqlinstance.Get(ctx, obj.TeamSlug, obj.Name, name)
}

func (r *teamInventoryCountsResolver) SQLInstances(ctx context.Context, obj *team.TeamInventoryCounts) (*sqlinstance.TeamInventoryCountSQLInstances, error) {
	return &sqlinstance.TeamInventoryCountSQLInstances{
		Total: len(sqlinstance.ListAllForTeam(ctx, obj.TeamSlug)),
	}, nil
}

func (r *Resolver) SqlDatabase() gengqlv1.SqlDatabaseResolver { return &sqlDatabaseResolver{r} }

func (r *Resolver) SqlInstance() gengqlv1.SqlInstanceResolver { return &sqlInstanceResolver{r} }

type (
	sqlDatabaseResolver struct{ *Resolver }
	sqlInstanceResolver struct{ *Resolver }
)
