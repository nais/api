package sqlinstance

import (
	"context"
	"errors"
	"fmt"
	"google.golang.org/api/googleapi"
	"sort"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/slug"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
)

func (c *Client) SqlInstance(ctx context.Context, env string, teamSlug slug.Slug, instanceName string) (*model.SQLInstance, error) {
	inf, exists := c.informers[env]
	if !exists {
		return nil, fmt.Errorf("unknown env: %s", env)
	}

	if inf.SqlInstanceInformer == nil {
		return nil, fmt.Errorf("SQL instance informer not supported in env: %q", env)
	}

	obj, err := inf.SqlInstanceInformer.Lister().ByNamespace(string(teamSlug)).Get(instanceName)
	if err != nil {
		return nil, fmt.Errorf("get SQL instance: %w", err)
	}

	instance, err := model.ToSqlInstance(obj.(*unstructured.Unstructured), env)
	if err != nil {
		return nil, err
	}

	metrics, err := c.metrics.metricsForSqlInstance(ctx, instance)
	if err != nil {
		return nil, err
	}
	instance.Metrics = metrics

	return instance, nil
}

func (c *Client) SqlInstances(ctx context.Context, teamSlug slug.Slug) ([]*model.SQLInstance, *model.SQLInstancesMetrics, error) {
	ret := make([]*model.SQLInstance, 0)

	for env, infs := range c.informers {
		if infs.SqlInstanceInformer == nil {
			continue
		}

		objs, err := infs.SqlInstanceInformer.Lister().ByNamespace(string(teamSlug)).List(labels.Everything())
		if err != nil {
			return nil, nil, c.error(err, "listing SQL instances")
		}

		for _, obj := range objs {
			instance, err := model.ToSqlInstance(obj.(*unstructured.Unstructured), env)
			if err != nil {
				return nil, nil, c.error(err, "converting to SQL instance model")
			}

			metrics, err := c.metrics.metricsForSqlInstance(ctx, instance) // instance => ws-test
			if err != nil {
				return nil, nil, err
			}
			instance.Metrics = metrics

			ret = append(ret, instance)
		}
	}
	sort.Slice(ret, func(i, j int) bool {
		return ret[i].ConnectionName < ret[j].ConnectionName
	})

	return ret, metricsSummary(ret), nil
}

func metricsSummary(instances []*model.SQLInstance) *model.SQLInstancesMetrics {
	var cost, cpuCores, cpuUtilization, diskUtilization, memoryUtilization float64
	var diskQuota, memoryQuota int

	for _, instance := range instances {
		cost += instance.Metrics.Cost
		cpuCores += instance.Metrics.CPU.Cores
		cpuUtilization += instance.Metrics.CPU.Utilization
		diskQuota += instance.Metrics.Disk.QuotaBytes
		diskUtilization += instance.Metrics.Disk.Utilization
		memoryQuota += instance.Metrics.Memory.QuotaBytes
		memoryUtilization += instance.Metrics.Memory.Utilization
	}

	numInstances := float64(len(instances))
	return &model.SQLInstancesMetrics{
		Cost: cost,
		CPU: model.SQLInstanceCPU{
			Cores: cpuCores,
			Utilization: func() float64 {
				if numInstances == 0 {
					return 0
				}
				return cpuUtilization / numInstances
			}(),
		},
		Disk: model.SQLInstanceDisk{
			QuotaBytes: diskQuota,
			Utilization: func() float64 {
				if numInstances == 0 {
					return 0
				}
				return diskUtilization / numInstances
			}(),
		},
		Memory: model.SQLInstanceMemory{
			QuotaBytes: memoryQuota,
			Utilization: func() float64 {
				if numInstances == 0 {
					return 0
				}
				return memoryUtilization / numInstances
			}(),
		},
	}
}

func (c *Client) SqlDatabase(sqlInstance *model.SQLInstance) (*model.SQLDatabase, error) {
	inf := c.informers[sqlInstance.Env.Name]
	if inf == nil {
		return nil, fmt.Errorf("unknown env: %s", sqlInstance.Env.Name)
	}

	objs, err := inf.SqlDatabaseInformer.Lister().ByNamespace(string(sqlInstance.GQLVars.TeamSlug)).List(labels.Everything())
	if err != nil {
		return nil, c.error(err, "listing SQL databases")
	}

	for _, obj := range objs {
		db, err := model.ToSqlDatabase(obj.(*unstructured.Unstructured), sqlInstance.Name)
		if err != nil {
			return nil, c.error(err, "converting to SQL database model")
		}

		if db != nil {
			return db, nil
		}
	}
	return nil, nil
}

func (c *Client) SqlUsers(ctx context.Context, sqlInstance *model.SQLInstance) ([]*model.SQLUser, error) {
	users, err := c.admin.GetUsers(ctx, sqlInstance.ProjectID, sqlInstance.Name)

	// TODO handle error in a better way
	if err != nil {
		var googleErr *googleapi.Error
		if errors.As(err, &googleErr) {
			if googleErr.Code == 400 {
				c.log.WithError(err).Info("could not get SQL users, instance not found or stopped")
				return nil, nil
			}
		}
		return nil, c.error(err, "getting SQL users")
	}

	ret := make([]*model.SQLUser, 0)
	for _, user := range users {
		ret = append(ret, &model.SQLUser{
			Name:           user.Name,
			Authentication: authentication(user.Type),
		})
	}
	return ret, nil
}

func authentication(t string) string {
	switch t {
	case "":
		return "Built-in"
	default:
		return t
	}
}

func (c *Client) error(err error, msg string) error {
	c.log.WithError(err).Error(msg)
	return fmt.Errorf("%s: %w", msg, err)
}
