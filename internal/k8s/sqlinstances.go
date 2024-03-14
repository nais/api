package k8s

import (
	"context"
	"fmt"
	"sort"

	"github.com/nais/api/internal/graph/model"
	sql_cnrm_cloud_google_com_v1beta1 "github.com/nais/liberator/pkg/apis/sql.cnrm.cloud.google.com/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

func (c *Client) SqlInstances(ctx context.Context, team string) ([]*model.SQLInstance, error) {
	ret := make([]*model.SQLInstance, 0)

	for env, infs := range c.informers {
		objs, err := infs.SqlInstanceInformer.Lister().ByNamespace(team).List(labels.Everything())
		if err != nil {
			return nil, c.error(ctx, err, "listing SQL instances")
		}

		for _, obj := range objs {
			instance, err := c.toSqlInstance(ctx, obj.(*unstructured.Unstructured), env)
			if err != nil {
				return nil, c.error(ctx, err, "converting to SQL instance model")
			}

			ret = append(ret, instance)
		}
	}
	sort.Slice(ret, func(i, j int) bool {
		return ret[i].Name < ret[j].Name
	})

	return ret, nil
}

func (c *Client) toSqlInstance(_ context.Context, u *unstructured.Unstructured, env string) (*model.SQLInstance, error) {
	sqlInstance := &sql_cnrm_cloud_google_com_v1beta1.SQLInstance{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, sqlInstance); err != nil {
		return nil, fmt.Errorf("converting to SQL instance: %w", err)
	}

	return &model.SQLInstance{
		Name:        sqlInstance.Name,
		Environment: env,
	}, nil
}
