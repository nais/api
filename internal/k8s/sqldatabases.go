package k8s

import (
	"context"
	"fmt"
	"sort"

	sql_cnrm_cloud_google_com_v1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/pkg/clients/generated/apis/sql/v1beta1"
	"github.com/nais/api/internal/graph/model"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

func (c *Client) SqlDatabases(ctx context.Context, sqlInstance *model.SQLInstance) ([]*model.SQLDatabase, error) {
	ret := make([]*model.SQLDatabase, 0)

	for _, infs := range c.informers {
		objs, err := infs.SqlDatabaseInformer.Lister().ByNamespace(sqlInstance.Team.Slug.String()).List(labels.Everything())
		if err != nil {
			return nil, c.error(ctx, err, "listing SQL databases")
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
