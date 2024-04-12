package sqlinstance

import (
	"context"
	"fmt"
	"sort"

	sql_cnrm_cloud_google_com_v1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/pkg/clients/generated/apis/sql/v1beta1"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/scalar"
	"github.com/nais/api/internal/slug"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	instance, err := inf.SqlInstanceInformer.Lister().ByNamespace(string(teamSlug)).Get(instanceName)
	if err != nil {
		return nil, fmt.Errorf("get SQL instance: %w", err)
	}

	return c.toSqlInstance(ctx, instance.(*unstructured.Unstructured), teamSlug, env)
}

func (c *Client) SqlInstances(ctx context.Context, teamSlug slug.Slug) ([]*model.SQLInstance, error) {
	ret := make([]*model.SQLInstance, 0)

	for env, infs := range c.informers {
		if infs.SqlInstanceInformer == nil {
			continue
		}

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
		return ret[i].ConnectionName < ret[j].ConnectionName
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
		BackupConfiguration: func(backupConfig *sql_cnrm_cloud_google_com_v1beta1.InstanceBackupConfiguration) *model.BackupConfiguration {
			if backupConfig == nil {
				return nil
			}
			backupCfg := &model.BackupConfiguration{}
			if backupConfig.Enabled != nil {
				backupCfg.Enabled = *backupConfig.Enabled
			}
			if backupConfig.StartTime != nil {
				backupCfg.StartTime = *backupConfig.StartTime
			}
			if backupConfig.BackupRetentionSettings != nil {
				backupCfg.RetainedBackups = backupConfig.BackupRetentionSettings.RetainedBackups
			}
			if backupConfig.PointInTimeRecoveryEnabled != nil {
				backupCfg.PointInTimeRecovery = *backupConfig.PointInTimeRecoveryEnabled
			}
			return backupCfg
		}(sqlInstance.Spec.Settings.BackupConfiguration),
		CascadingDelete: sqlInstance.GetAnnotations()["cnrm.cloud.google.com/deletion-policy"] != "abandon",
		Type:            *sqlInstance.Spec.DatabaseVersion,
		ConnectionName:  *sqlInstance.Status.ConnectionName,
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
			PublicIPAddress: sqlInstance.Status.PublicIpAddress,
		},
		MaintenanceWindow: func(window *sql_cnrm_cloud_google_com_v1beta1.InstanceMaintenanceWindow) *model.MaintenanceWindow {
			if window == nil || window.Day == nil || window.Hour == nil {
				return nil
			}
			return &model.MaintenanceWindow{
				Day:  *window.Day,
				Hour: *window.Hour,
			}
		}(sqlInstance.Spec.Settings.MaintenanceWindow),
		Flags: func() []*model.Flag {
			ret := make([]*model.Flag, 0)
			for _, flag := range sqlInstance.Spec.Settings.DatabaseFlags {
				ret = append(ret, &model.Flag{
					Name:  flag.Name,
					Value: flag.Value,
				})
			}
			return ret
		}(),
		MaintenanceVersion: sqlInstance.Spec.MaintenanceVersion,
		ProjectID:          projectId,
		Tier:               sqlInstance.Spec.Settings.Tier,
		HighAvailability:   equals(sqlInstance.Spec.Settings.AvailabilityType, "REGIONAL"),
		GQLVars: model.SQLInstanceGQLVars{
			TeamSlug:    teamSlug,
			Labels:      sqlInstance.GetLabels(),
			Annotations: sqlInstance.GetAnnotations(),
			OwnerReference: func(refs []v1.OwnerReference) *v1.OwnerReference {
				if len(refs) == 0 {
					return nil
				}

				for _, o := range refs {
					if o.Kind == "Naisjob" || o.Kind == "Application" {
						return &v1.OwnerReference{
							APIVersion: o.APIVersion,
							Kind:       o.Kind,
							Name:       o.Name,
							UID:        o.UID,
						}
					}
				}
				return nil
			}(sqlInstance.OwnerReferences),
		},
	}, nil
}

func (c *Client) SqlDatabases(ctx context.Context, sqlInstance *model.SQLInstance) ([]*model.SQLDatabase, error) {
	ret := make([]*model.SQLDatabase, 0)

	inf := c.informers[sqlInstance.Env.Name]
	if inf == nil {
		return nil, fmt.Errorf("unknown env: %s", sqlInstance.Env.Name)
	}

	objs, err := inf.SqlDatabaseInformer.Lister().ByNamespace(string(sqlInstance.GQLVars.TeamSlug)).List(labels.Everything())
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

func (c *Client) error(_ context.Context, err error, msg string) error {
	c.log.WithError(err).Error(msg)
	return fmt.Errorf("%s: %w", msg, err)
}

func equals(s *string, eq string) bool {
	return s != nil && *s == eq
}
