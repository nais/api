package k8s

import (
	"context"
	"fmt"
	"sort"

	sql_cnrm_cloud_google_com_v1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/pkg/clients/generated/apis/sql/v1beta1"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/scalar"
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

	instance, err := inf.SqlInstanceInformer.Lister().ByNamespace(string(teamSlug)).Get(instanceName)
	if err != nil {
		return nil, fmt.Errorf("get SQL instance: %w", err)
	}

	return c.toSqlInstance(ctx, instance.(*unstructured.Unstructured), teamSlug, env)
}

func (c *Client) SqlInstances(ctx context.Context, teamSlug slug.Slug) ([]*model.SQLInstance, error) {
	ret := make([]*model.SQLInstance, 0)

	for env, infs := range c.informers {
		objs, err := infs.SqlInstanceInformer.Lister().ByNamespace(string(teamSlug)).List(labels.Everything())
		if err != nil {
			return nil, c.error(ctx, err, "listing SQL instances")
		}

		for _, obj := range objs {
			instance, err := c.toSqlInstance(ctx, obj.(*unstructured.Unstructured), teamSlug, env)
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

func (c *Client) toSqlInstance(_ context.Context, u *unstructured.Unstructured, teamSlug slug.Slug, env string) (*model.SQLInstance, error) {
	sqlInstance := &sql_cnrm_cloud_google_com_v1beta1.SQLInstance{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, sqlInstance); err != nil {
		return nil, fmt.Errorf("converting to SQL instance: %w", err)
	}
	projectId := sqlInstance.GetAnnotations()["cnrm.cloud.google.com/project-id"]
	if projectId == "" {
		return nil, fmt.Errorf("missing project ID annotation")
	}

	return &model.SQLInstance{
		ID:   scalar.SqlInstanceIdent("sqlInstance_" + env + "_" + sqlInstance.GetNamespace() + "_" + sqlInstance.GetName()),
		Name: sqlInstance.Name,
		Env: model.Env{
			Name: env,
			Team: teamSlug.String(),
		},
		BackupConfiguration: func() *model.BackupConfiguration {
			if sqlInstance.Spec.Settings.BackupConfiguration == nil {
				return nil
			}
			backupCfg := &model.BackupConfiguration{}
			if sqlInstance.Spec.Settings.BackupConfiguration.BackupRetentionSettings != nil {
				backupCfg.RetainedBackups = sqlInstance.Spec.Settings.BackupConfiguration.BackupRetentionSettings.RetainedBackups
			}
			if sqlInstance.Spec.Settings.BackupConfiguration.Enabled != nil {
				backupCfg.Enabled = *sqlInstance.Spec.Settings.BackupConfiguration.Enabled
			}
			if sqlInstance.Spec.Settings.BackupConfiguration.StartTime != nil {
				backupCfg.StartTime = *sqlInstance.Spec.Settings.BackupConfiguration.StartTime
			}
			return backupCfg
		}(),
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
		ProjectID:        projectId,
		Tier:             sqlInstance.Spec.Settings.Tier,
		HighAvailability: equals(sqlInstance.Spec.Settings.AvailabilityType, "REGIONAL"),
		GQLVars: model.SQLInstanceGQLVars{
			TeamSlug:    teamSlug,
			Labels:      sqlInstance.GetLabels(),
			Annotations: sqlInstance.GetAnnotations(),
		},
	}, nil
}

func equals(s *string, eq string) bool {
	return s != nil && *s == eq
}