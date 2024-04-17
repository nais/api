package sqlinstance

import (
	"context"
	"fmt"
	"sort"

	sql_cnrm_cloud_google_com_v1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/pkg/clients/generated/apis/sql/v1beta1"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/slug"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
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
			Cores:       cpuCores,
			Utilization: cpuUtilization / numInstances,
		},
		Disk: model.SQLInstanceDisk{
			QuotaBytes:  diskQuota,
			Utilization: diskUtilization / numInstances,
		},
		Memory: model.SQLInstanceMemory{
			QuotaBytes:  memoryQuota,
			Utilization: memoryUtilization / numInstances,
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
		sqlDatabase := &sql_cnrm_cloud_google_com_v1beta1.SQLDatabase{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.(*unstructured.Unstructured).Object, sqlDatabase); err != nil {
			return nil, fmt.Errorf("converting to SQL database: %w", err)
		}

		if sqlDatabase.Spec.InstanceRef.Name != sqlInstance.Name {
			continue
		}

		return &model.SQLDatabase{
			Name: sqlDatabase.Name,
		}, nil
	}

	return nil, nil
}

func (c *Client) error(err error, msg string) error {
	c.log.WithError(err).Error(msg)
	return fmt.Errorf("%s: %w", msg, err)
}
