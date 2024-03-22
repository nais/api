package k8s

import (
	"context"
	"fmt"
	"sort"

	sql_cnrm_cloud_google_com_v1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/pkg/clients/generated/apis/sql/v1beta1"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/scalar"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

func (c *Client) SqlInstance(ctx context.Context, env string, team *model.Team, instanceName string) (*model.SQLInstance, error) {
	inf, exists := c.informers[env]
	if !exists {
		return nil, fmt.Errorf("unknown env: %s", env)
	}

	instance, err := inf.SqlInstanceInformer.Lister().ByNamespace(team.Slug.String()).Get(instanceName)
	if err != nil {
		return nil, fmt.Errorf("get SQL instance: %w", err)
	}

	return c.toSqlInstance(ctx, instance.(*unstructured.Unstructured), team, env)
}

func (c *Client) SqlInstances(ctx context.Context, team *model.Team) ([]*model.SQLInstance, error) {
	ret := make([]*model.SQLInstance, 0)

	for env, infs := range c.informers {
		objs, err := infs.SqlInstanceInformer.Lister().ByNamespace(team.Slug.String()).List(labels.Everything())
		if err != nil {
			return nil, c.error(ctx, err, "listing SQL instances")
		}

		for _, obj := range objs {
			instance, err := c.toSqlInstance(ctx, obj.(*unstructured.Unstructured), team, env)
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

func (c *Client) toSqlInstance(_ context.Context, u *unstructured.Unstructured, team *model.Team, env string) (*model.SQLInstance, error) {
	sqlInstance := &sql_cnrm_cloud_google_com_v1beta1.SQLInstance{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, sqlInstance); err != nil {
		return nil, fmt.Errorf("converting to SQL instance: %w", err)
	}

	return &model.SQLInstance{
		ID:   scalar.SqlInstanceIdent("sqlInstance_" + env + "_" + sqlInstance.GetNamespace() + "_" + sqlInstance.GetName()),
		Name: sqlInstance.Name,
		Env: model.Env{
			Name: env,
			Team: team.Slug.String(),
		},
		BackupConfiguration: &model.BackupConfiguration{
			Enabled:         *sqlInstance.Spec.Settings.BackupConfiguration.Enabled,
			StartTime:       *sqlInstance.Spec.Settings.BackupConfiguration.StartTime,
			RetainedBackups: sqlInstance.Spec.Settings.BackupConfiguration.BackupRetentionSettings.RetainedBackups,
		},
		Team:           team,
		Type:           *sqlInstance.Spec.DatabaseVersion,
		ConnectionName: *sqlInstance.Status.ConnectionName,
		Status: model.SQLInstanceStatus{
			Conditions: func() []*model.SQLInstanceCondition {
				ret := make([]*model.SQLInstanceCondition, 0)
				for _, condition := range sqlInstance.Status.Conditions {
					ret = append(ret, &model.SQLInstanceCondition{
						Type:    condition.Type,
						Status:  string(condition.Status),
						Reason:  condition.Reason,
						Message: condition.Message,
					})
				}
				return ret
			}(),
		},
		Tier:             sqlInstance.Spec.Settings.Tier,
		HighAvailability: equals(sqlInstance.Spec.Settings.AvailabilityType, "REGIONAL"),
		GQLVars: model.SQLInstanceGQLVars{
			Labels:      sqlInstance.GetLabels(),
			Annotations: sqlInstance.GetAnnotations(),
		},
	}, nil
}

func equals(s *string, eq string) bool {
	return s != nil && *s == eq
}
