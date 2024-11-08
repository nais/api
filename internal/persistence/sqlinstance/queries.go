package sqlinstance

import (
	"context"
	"errors"
	"fmt"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/search"
	"github.com/nais/api/internal/slug"
	nais_io_v1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
	"google.golang.org/api/googleapi"
)

func GetByIdent(ctx context.Context, id ident.Ident) (*SQLInstance, error) {
	teamSlug, environmentName, sqlInstanceName, err := parseIdent(id)
	if err != nil {
		return nil, err
	}

	return Get(ctx, teamSlug, environmentName, sqlInstanceName)
}

func Get(ctx context.Context, teamSlug slug.Slug, environmentName, sqlInstanceName string) (*SQLInstance, error) {
	return fromContext(ctx).sqlInstanceWatcher.Get(environmentName, teamSlug.String(), sqlInstanceName)
}

func GetDatabaseByIdent(ctx context.Context, id ident.Ident) (*SQLDatabase, error) {
	teamSlug, environmentName, sqlInstanceName, err := parseIdent(id)
	if err != nil {
		return nil, err
	}

	return GetDatabase(ctx, teamSlug, environmentName, sqlInstanceName)
}

func GetDatabase(ctx context.Context, teamSlug slug.Slug, environmentName, sqlInstanceName string) (*SQLDatabase, error) {
	return fromContext(ctx).sqlDatabaseWatcher.Get(environmentName, teamSlug.String(), sqlInstanceName)
}

func ListForWorkload(ctx context.Context, teamSlug slug.Slug, environmentName string, references []nais_io_v1.CloudSqlInstance, orderBy *SQLInstanceOrder) (*SQLInstanceConnection, error) {
	all := fromContext(ctx).sqlInstanceWatcher.GetByNamespace(teamSlug.String())

	ret := make([]*SQLInstance, 0)

	for _, ref := range references {
		for _, d := range all {
			if d.Obj.Name == ref.Name && d.Obj.EnvironmentName == environmentName {
				ret = append(ret, d.Obj)
			}
		}
	}

	orderSQLInstances(ctx, ret, orderBy)

	return pagination.NewConnectionWithoutPagination(ret), nil
}

func Search(ctx context.Context, q string) ([]*search.Result, error) {
	apps := fromContext(ctx).sqlInstanceWatcher.All()

	ret := make([]*search.Result, 0)
	for _, app := range apps {
		rank := search.Match(q, app.Obj.Name)
		if search.Include(rank) {
			ret = append(ret, &search.Result{
				Rank: rank,
				Node: app.Obj,
			})
		}
	}

	return ret, nil
}

func orderSQLInstances(ctx context.Context, instances []*SQLInstance, orderBy *SQLInstanceOrder) {
	if orderBy == nil {
		orderBy = &SQLInstanceOrder{
			Field:     SQLInstanceOrderFieldName,
			Direction: model.OrderDirectionAsc,
		}
	}

	SortFilterSQLInstance.Sort(ctx, instances, orderBy.Field, orderBy.Direction)
}

func ListForTeam(ctx context.Context, teamSlug slug.Slug, page *pagination.Pagination, orderBy *SQLInstanceOrder) (*SQLInstanceConnection, error) {
	all := ListAllForTeam(ctx, teamSlug)
	orderSQLInstances(ctx, all, orderBy)

	instances := pagination.Slice(all, page)
	return pagination.NewConnection(instances, page, len(all)), nil
}

func ListAllForTeam(ctx context.Context, teamSlug slug.Slug) []*SQLInstance {
	all := fromContext(ctx).sqlInstanceWatcher.GetByNamespace(teamSlug.String())
	return watcher.Objects(all)
}

func ListSQLInstanceUsers(ctx context.Context, sqlInstance *SQLInstance, page *pagination.Pagination, orderBy *SQLInstanceUserOrder) (*SQLInstanceUserConnection, error) {
	adminUsers, err := fromContext(ctx).sqlAdminService.GetUsers(ctx, sqlInstance.ProjectID, sqlInstance.Name)
	if err != nil {
		var googleErr *googleapi.Error
		if errors.As(err, &googleErr) && googleErr.Code == 400 {
			// TODO: This was handled in the legacy code, keep it for now. Log?
			return nil, nil
		}
		return nil, fmt.Errorf("getting SQL users")
	}

	all := make([]*SQLInstanceUser, len(adminUsers))
	for i, user := range adminUsers {
		all[i] = toSQLInstanceUser(user)
	}

	if orderBy == nil {
		orderBy = &SQLInstanceUserOrder{
			Field:     SQLInstanceUserOrderFieldName,
			Direction: model.OrderDirectionAsc,
		}
	}

	SortFilterSQLInstanceUser.Sort(ctx, all, orderBy.Field, orderBy.Direction)

	users := pagination.Slice(all, page)
	return pagination.NewConnection(users, page, len(all)), nil
}

func GetState(ctx context.Context, project, instance string) (SQLInstanceState, error) {
	i, err := fromContext(ctx).remoteSQLInstance.Load(ctx, instanceKey{projectID: project, name: instance})
	if err != nil {
		var googleErr *googleapi.Error
		if errors.As(err, &googleErr) && googleErr.Code == 404 {
			return SQLInstanceStateUnspecified, nil
		}
		return "", err
	}
	return SQLInstanceState(i.State), nil
}

func MetricsFor(ctx context.Context, projectID, name string) (*SQLInstanceMetrics, error) {
	return &SQLInstanceMetrics{
		InstanceName: name,
		ProjectID:    projectID,
	}, nil
}

func CPUForInstance(ctx context.Context, projectID, instance string) (*SQLInstanceCPU, error) {
	return fromContext(ctx).sqlMetricsService.cpuForSQLInstance(ctx, projectID, instance)
}

func MemoryForInstance(ctx context.Context, projectID, instance string) (*SQLInstanceMemory, error) {
	return fromContext(ctx).sqlMetricsService.memoryForSQLInstance(ctx, projectID, instance)
}

func DiskForInstance(ctx context.Context, projectID, instance string) (*SQLInstanceDisk, error) {
	return fromContext(ctx).sqlMetricsService.diskForSQLInstance(ctx, projectID, instance)
}

func TeamSummaryCPU(ctx context.Context, projectID string) (*TeamServiceUtilizationSQLInstancesCPU, error) {
	return fromContext(ctx).sqlMetricsService.teamSummaryCPU(ctx, projectID)
}

func TeamSummaryMemory(ctx context.Context, projectID string) (*TeamServiceUtilizationSQLInstancesMemory, error) {
	return fromContext(ctx).sqlMetricsService.teamSummaryMemory(ctx, projectID)
}

func TeamSummaryDisk(ctx context.Context, projectID string) (*TeamServiceUtilizationSQLInstancesDisk, error) {
	return fromContext(ctx).sqlMetricsService.teamSummaryDisk(ctx, projectID)
}
