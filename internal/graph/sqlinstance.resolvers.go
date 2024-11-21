package graph

import (
	"context"
	"errors"

	"github.com/davecgh/go-spew/spew"
	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/persistence/sqlinstance"
	"github.com/nais/api/internal/team"
	"github.com/nais/api/internal/utilization"
	"github.com/nais/api/internal/workload"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
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
	return pagination.NewConnection(ret, page, len(obj.Flags)), nil
}

func (r *sqlInstanceResolver) Users(ctx context.Context, obj *sqlinstance.SQLInstance, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *sqlinstance.SQLInstanceUserOrder) (*pagination.Connection[*sqlinstance.SQLInstanceUser], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return sqlinstance.ListSQLInstanceUsers(ctx, obj, page, orderBy)
}

func (r *sqlInstanceResolver) Metrics(ctx context.Context, obj *sqlinstance.SQLInstance) (*sqlinstance.SQLInstanceMetrics, error) {
	return sqlinstance.MetricsFor(ctx, obj.ProjectID, obj.Name)
}

func (r *sqlInstanceResolver) State(ctx context.Context, obj *sqlinstance.SQLInstance) (sqlinstance.SQLInstanceState, error) {
	return sqlinstance.GetState(ctx, obj.ProjectID, obj.Name)
}

func (r *sqlInstanceMetricsResolver) CPU(ctx context.Context, obj *sqlinstance.SQLInstanceMetrics) (*sqlinstance.SQLInstanceCPU, error) {
	return sqlinstance.CPUForInstance(ctx, obj.ProjectID, obj.InstanceName)
}

func (r *sqlInstanceMetricsResolver) Memory(ctx context.Context, obj *sqlinstance.SQLInstanceMetrics) (*sqlinstance.SQLInstanceMemory, error) {
	return sqlinstance.MemoryForInstance(ctx, obj.ProjectID, obj.InstanceName)
}

func (r *sqlInstanceMetricsResolver) Disk(ctx context.Context, obj *sqlinstance.SQLInstanceMetrics) (*sqlinstance.SQLInstanceDisk, error) {
	return sqlinstance.DiskForInstance(ctx, obj.ProjectID, obj.InstanceName)
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

func (r *teamServiceUtilizationResolver) SQLInstances(ctx context.Context, obj *utilization.TeamServiceUtilization) (*sqlinstance.TeamServiceUtilizationSQLInstances, error) {
	envs, err := team.ListTeamEnvironments(ctx, obj.TeamSlug)
	if err != nil {
		return nil, err
	}

	var gcpProjectIDs []string
	for _, env := range envs {
		if env.GCPProjectID != nil && *env.GCPProjectID != "" {
			gcpProjectIDs = append(gcpProjectIDs, *env.GCPProjectID)
		}
	}

	return &sqlinstance.TeamServiceUtilizationSQLInstances{
		TeamSlug:   obj.TeamSlug,
		ProjectIDs: gcpProjectIDs,
	}, nil
}

func (r *teamServiceUtilizationSqlInstancesResolver) CPU(ctx context.Context, obj *sqlinstance.TeamServiceUtilizationSQLInstances) (*sqlinstance.TeamServiceUtilizationSQLInstancesCPU, error) {
	spew.Dump(obj)
	ret := &sqlinstance.TeamServiceUtilizationSQLInstancesCPU{}
	for _, projectID := range obj.ProjectIDs {
		r, err := sqlinstance.TeamSummaryCPU(ctx, projectID)
		if err != nil {
			return nil, err
		}
		ret.Used += r.Used
		ret.Requested += r.Requested
	}

	if ret.Requested > 0 {
		ret.Utilization = ret.Used / ret.Requested
	}

	return ret, nil
}

func (r *teamServiceUtilizationSqlInstancesResolver) Memory(ctx context.Context, obj *sqlinstance.TeamServiceUtilizationSQLInstances) (*sqlinstance.TeamServiceUtilizationSQLInstancesMemory, error) {
	ret := &sqlinstance.TeamServiceUtilizationSQLInstancesMemory{}
	for _, projectID := range obj.ProjectIDs {
		r, err := sqlinstance.TeamSummaryMemory(ctx, projectID)
		if err != nil {
			return nil, err
		}
		ret.Used += r.Used
		ret.Requested += r.Requested
	}

	if ret.Requested > 0 {
		ret.Utilization = float64(ret.Used) / float64(ret.Requested)
	}

	return ret, nil
}

func (r *teamServiceUtilizationSqlInstancesResolver) Disk(ctx context.Context, obj *sqlinstance.TeamServiceUtilizationSQLInstances) (*sqlinstance.TeamServiceUtilizationSQLInstancesDisk, error) {
	ret := &sqlinstance.TeamServiceUtilizationSQLInstancesDisk{}
	for _, projectID := range obj.ProjectIDs {
		r, err := sqlinstance.TeamSummaryDisk(ctx, projectID)
		if err != nil {
			return nil, err
		}
		ret.Used += r.Used
		ret.Requested += r.Requested
	}

	if ret.Requested > 0 {
		ret.Utilization = float64(ret.Used) / float64(ret.Requested)
	}

	return ret, nil
}

func (r *Resolver) SqlDatabase() gengql.SqlDatabaseResolver { return &sqlDatabaseResolver{r} }

func (r *Resolver) SqlInstance() gengql.SqlInstanceResolver { return &sqlInstanceResolver{r} }

func (r *Resolver) SqlInstanceMetrics() gengql.SqlInstanceMetricsResolver {
	return &sqlInstanceMetricsResolver{r}
}

func (r *Resolver) TeamServiceUtilizationSqlInstances() gengql.TeamServiceUtilizationSqlInstancesResolver {
	return &teamServiceUtilizationSqlInstancesResolver{r}
}

type (
	sqlDatabaseResolver                        struct{ *Resolver }
	sqlInstanceResolver                        struct{ *Resolver }
	sqlInstanceMetricsResolver                 struct{ *Resolver }
	teamServiceUtilizationSqlInstancesResolver struct{ *Resolver }
)
