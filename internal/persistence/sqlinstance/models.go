package sqlinstance

import (
	"fmt"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/clients/generated/apis/k8s/v1alpha1"
	sql_cnrm_cloud_google_com_v1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/pkg/clients/generated/apis/sql/v1beta1"
	"github.com/nais/api/internal/graphv1/ident"
	"github.com/nais/api/internal/graphv1/pagination"
	"github.com/nais/api/internal/persistence"
	"github.com/nais/api/internal/slug"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
)

type (
	SQLInstanceConnection     = pagination.Connection[*SQLInstance]
	SQLInstanceEdge           = pagination.Edge[*SQLInstance]
	SQLInstanceFlagConnection = pagination.Connection[*SQLInstanceFlag]
	SQLInstanceFlagEdge       = pagination.Edge[*SQLInstanceFlag]
)

type SQLDatabase struct {
	Name            string    `json:"name"`
	Charset         *string   `json:"charset,omitempty"`
	Collation       *string   `json:"collation,omitempty"`
	DeletionPolicy  *string   `json:"deletionPolicy,omitempty"`
	Healthy         bool      `json:"healthy"`
	SQLInstanceName string    `json:"-"`
	EnvironmentName string    `json:"-"`
	TeamSlug        slug.Slug `json:"-"`
}

func (SQLDatabase) IsPersistence() {}

func (SQLDatabase) IsNode() {}

func (d SQLDatabase) ID() ident.Ident {
	return newDatabaseIdent(d.TeamSlug, d.EnvironmentName, d.SQLInstanceName)
}

type SQLInstance struct {
	Name                string                          `json:"name"`
	CascadingDelete     bool                            `json:"cascadingDelete"`
	ConnectionName      *string                         `json:"connectionName,omitempty"`
	DiskAutoresize      *bool                           `json:"diskAutoresize,omitempty"`
	DiskAutoresizeLimit *int64                          `json:"diskAutoresizeLimit,omitempty"`
	HighAvailability    bool                            `json:"highAvailability"`
	Healthy             bool                            `json:"healthy"`
	MaintenanceVersion  *string                         `json:"maintenanceVersion,omitempty"`
	MaintenanceWindow   *SQLInstanceMaintenanceWindow   `json:"maintenanceWindow,omitempty"`
	BackupConfiguration *SQLInstanceBackupConfiguration `json:"backupConfiguration,omitempty"`
	ProjectID           string                          `json:"projectId"`
	Tier                string                          `json:"tier"`
	Version             *string                         `json:"version,omitempty"`
	Status              SQLInstanceStatus               `json:"status"`
	Flags               []*SQLInstanceFlag              `json:"-"`
	EnvironmentName     string                          `json:"-"`
	TeamSlug            slug.Slug                       `json:"-"`
	OwnerReference      *metav1.OwnerReference          `json:"-"`
}

func (SQLInstance) IsPersistence() {}

func (SQLInstance) IsNode() {}

func (i SQLInstance) ID() ident.Ident {
	return newIdent(i.TeamSlug, i.EnvironmentName, i.Name)
}

type SQLInstanceBackupConfiguration struct {
	Enabled                     *bool   `json:"enabled,omitempty"`
	StartTime                   *string `json:"startTime,omitempty"`
	RetainedBackups             *int64  `json:"retainedBackups,omitempty"`
	PointInTimeRecovery         *bool   `json:"pointInTimeRecovery,omitempty"`
	TransactionLogRetentionDays *int64  `json:"transactionLogRetentionDays,omitempty"`
}

type SQLInstanceMaintenanceWindow struct {
	Day  int64 `json:"day"`
	Hour int64 `json:"hour"`
}

type SQLInstanceStatus struct {
	PublicIPAddress  *string `json:"publicIpAddress,omitempty"`
	PrivateIPAddress *string `json:"privateIpAddress,omitempty"`
}

type SQLInstanceFlag struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func healthy(conds []v1alpha1.Condition) bool {
	for _, cond := range conds {
		if cond.Type == string(corev1.PodReady) && cond.Reason == "UpToDate" && cond.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func toSQLInstanceBackupConfiguration(cfg *sql_cnrm_cloud_google_com_v1beta1.InstanceBackupConfiguration) *SQLInstanceBackupConfiguration {
	if cfg == nil {
		return nil
	}

	ret := &SQLInstanceBackupConfiguration{
		Enabled:                     cfg.Enabled,
		StartTime:                   cfg.StartTime,
		PointInTimeRecovery:         cfg.PointInTimeRecoveryEnabled,
		TransactionLogRetentionDays: cfg.TransactionLogRetentionDays,
	}

	if cfg.BackupRetentionSettings != nil {
		ret.RetainedBackups = &cfg.BackupRetentionSettings.RetainedBackups
	}

	return ret
}

func toSQLInstanceStatus(status sql_cnrm_cloud_google_com_v1beta1.SQLInstanceStatus) SQLInstanceStatus {
	ret := SQLInstanceStatus{
		PublicIPAddress: status.PublicIpAddress,
	}

	for _, ip := range status.IpAddress {
		if ip.Type != nil && *ip.Type == "PRIVATE" {
			ret.PrivateIPAddress = ip.IpAddress
		}
	}

	return ret
}

func toSQLInstanceMaintenanceWindow(window *sql_cnrm_cloud_google_com_v1beta1.InstanceMaintenanceWindow) *SQLInstanceMaintenanceWindow {
	if window == nil || window.Day == nil || window.Hour == nil {
		return nil
	}
	return &SQLInstanceMaintenanceWindow{
		Day:  *window.Day,
		Hour: *window.Hour,
	}
}

func toSQLInstanceFlags(flags []sql_cnrm_cloud_google_com_v1beta1.InstanceDatabaseFlags) []*SQLInstanceFlag {
	ret := make([]*SQLInstanceFlag, len(flags))
	for i, flag := range flags {
		ret[i] = &SQLInstanceFlag{
			Name:  flag.Name,
			Value: flag.Value,
		}
	}
	return ret
}

func toSQLDatabase(u *unstructured.Unstructured, environmentName, sqlInstanceName string) (*SQLDatabase, error) {
	obj := &sql_cnrm_cloud_google_com_v1beta1.SQLDatabase{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, obj); err != nil {
		return nil, fmt.Errorf("converting to SQL database: %w", err)
	}

	// TODO: Investigate if this is the correct way to identify the SQL database of an SQL instance
	// TODO: Does not handle abandoned databases
	if obj.Spec.InstanceRef.Name != sqlInstanceName {
		return nil, nil
	}

	return &SQLDatabase{
		Name:            ptr.Deref(obj.Spec.ResourceID, obj.ObjectMeta.Name),
		Charset:         obj.Spec.Charset,
		Collation:       obj.Spec.Collation,
		DeletionPolicy:  obj.Spec.DeletionPolicy,
		Healthy:         healthy(obj.Status.Conditions),
		SQLInstanceName: sqlInstanceName,
		EnvironmentName: environmentName,
		TeamSlug:        slug.Slug(obj.GetNamespace()),
	}, nil
}

func toSQLInstance(u *unstructured.Unstructured, environmentName string) (*SQLInstance, error) {
	obj := &sql_cnrm_cloud_google_com_v1beta1.SQLInstance{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, obj); err != nil {
		return nil, fmt.Errorf("converting to SQL instance: %w", err)
	}

	projectId := obj.GetAnnotations()["cnrm.cloud.google.com/project-id"]
	if projectId == "" {
		return nil, fmt.Errorf("missing project ID annotation")
	}

	return &SQLInstance{
		Name:                obj.Name,
		CascadingDelete:     obj.GetAnnotations()["cnrm.cloud.google.com/deletion-policy"] != "abandon",
		ConnectionName:      obj.Status.ConnectionName,
		DiskAutoresize:      obj.Spec.Settings.DiskAutoresize,
		DiskAutoresizeLimit: obj.Spec.Settings.DiskAutoresizeLimit,
		HighAvailability:    obj.Spec.Settings.AvailabilityType != nil && *obj.Spec.Settings.AvailabilityType == "REGIONAL",
		Healthy:             healthy(obj.Status.Conditions),
		MaintenanceVersion:  obj.Spec.MaintenanceVersion,
		MaintenanceWindow:   toSQLInstanceMaintenanceWindow(obj.Spec.Settings.MaintenanceWindow),
		BackupConfiguration: toSQLInstanceBackupConfiguration(obj.Spec.Settings.BackupConfiguration),
		ProjectID:           projectId,
		Tier:                obj.Spec.Settings.Tier,
		Version:             obj.Spec.DatabaseVersion,
		Status:              toSQLInstanceStatus(obj.Status),
		EnvironmentName:     environmentName,
		TeamSlug:            slug.Slug(obj.GetNamespace()),
		OwnerReference:      persistence.OwnerReference(obj.OwnerReferences),
		Flags:               toSQLInstanceFlags(obj.Spec.Settings.DatabaseFlags),
	}, nil
}
