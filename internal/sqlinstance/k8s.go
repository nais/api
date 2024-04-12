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

func (c *Client) SqlInstance(env string, teamSlug slug.Slug, instanceName string) (*model.SQLInstance, error) {
	inf, exists := c.informers[env]
	if !exists {
		return nil, fmt.Errorf("unknown env: %s", env)
	}

	if inf.SqlInstanceInformer == nil {
		return nil, fmt.Errorf("SQL instance informer not supported in env: %q", env)
	}

	instance, err := inf.SqlInstanceInformer.Lister().ByNamespace(string(teamSlug)).Get(instanceName)
	if err != nil {
		return nil, fmt.Errorf("get SQL instance: %w", err)
	}

	return model.ToSqlInstance(instance.(*unstructured.Unstructured), env)
}

func (c *Client) SqlInstances(teamSlug slug.Slug) ([]*model.SQLInstance, error) {
	ret := make([]*model.SQLInstance, 0)

	for env, infs := range c.informers {
		if infs.SqlInstanceInformer == nil {
			continue
		}

		objs, err := infs.SqlInstanceInformer.Lister().ByNamespace(string(teamSlug)).List(labels.Everything())
		if err != nil {
			return nil, c.error(err, "listing SQL instances")
		}

		for _, obj := range objs {
			instance, err := model.ToSqlInstance(obj.(*unstructured.Unstructured), env)
			if err != nil {
				return nil, c.error(err, "converting to SQL instance model")
			}

			ret = append(ret, instance)
		}
	}
	sort.Slice(ret, func(i, j int) bool {
		return ret[i].ConnectionName < ret[j].ConnectionName
	})

	return ret, nil
}

func (c *Client) SqlDatabases(ctx context.Context, sqlInstance *model.SQLInstance) ([]*model.SQLDatabase, error) {
	ret := make([]*model.SQLDatabase, 0)

	inf := c.informers[sqlInstance.Env.Name]
	if inf == nil {
		return nil, fmt.Errorf("unknown env: %s", sqlInstance.Env.Name)
	}

	objs, err := inf.SqlDatabaseInformer.Lister().ByNamespace(string(sqlInstance.GQLVars.TeamSlug)).List(labels.Everything())
	if err != nil {
		return nil, c.error(err, "listing SQL databases")
	}

	for _, obj := range objs {
		db := obj.(*unstructured.Unstructured)
		sqlDatabase := &sql_cnrm_cloud_google_com_v1beta1.SQLDatabase{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(db.Object, sqlDatabase); err != nil {
			return nil, fmt.Errorf("converting to SQL database: %w", err)
		}

		if sqlDatabase.Spec.InstanceRef.Name != sqlInstance.Name {
			continue
		}

		ret = append(ret, c.toSqlDatabase(sqlDatabase))
	}
	sort.Slice(ret, func(i, j int) bool {
		return ret[i].Name < ret[j].Name
	})

	return ret, nil
}

func (c *Client) toSqlDatabase(sqlDatabase *sql_cnrm_cloud_google_com_v1beta1.SQLDatabase) *model.SQLDatabase {
	return &model.SQLDatabase{
		Name: sqlDatabase.Name,
	}
}

func (c *Client) error(err error, msg string) error {
	c.log.WithError(err).Error(msg)
	return fmt.Errorf("%s: %w", msg, err)
}
