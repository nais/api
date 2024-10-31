package sqlinstance

import (
	"fmt"
	"io"
	"strconv"

	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/clients/generated/apis/k8s/v1alpha1"
	sql_cnrm_cloud_google_com_v1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/pkg/clients/generated/apis/sql/v1beta1"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v1/graphv1/ident"
	"github.com/nais/api/internal/v1/graphv1/modelv1"
	"github.com/nais/api/internal/v1/graphv1/pagination"
	"github.com/nais/api/internal/v1/workload"
	"google.golang.org/api/sqladmin/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
)

type (
	SQLInstanceConnection     = pagination.Connection[*SQLInstance]
	SQLInstanceEdge           = pagination.Edge[*SQLInstance]
	SQLInstanceFlagConnection = pagination.Connection[*SQLInstanceFlag]
	SQLInstanceFlagEdge       = pagination.Edge[*SQLInstanceFlag]
	SQLInstanceUserConnection = pagination.Connection[*SQLInstanceUser]
	SQLInstanceUserEdge       = pagination.Edge[*SQLInstanceUser]
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
func (SQLDatabase) IsNode()        {}

func (d *SQLDatabase) GetName() string { return d.Name }

func (d *SQLDatabase) GetNamespace() string { return d.TeamSlug.String() }

func (d *SQLDatabase) GetLabels() map[string]string { return nil }

func (d *SQLDatabase) GetObjectKind() schema.ObjectKind {
	return schema.EmptyObjectKind
}

func (d *SQLDatabase) DeepCopyObject() runtime.Object {
	return d
}

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
	ProjectID           string                          `json:"projectID"`
	Tier                string                          `json:"tier"`
	Version             *string                         `json:"version,omitempty"`
	Status              *SQLInstanceStatus              `json:"status"`
	Flags               []*SQLInstanceFlag              `json:"-"`
	EnvironmentName     string                          `json:"-"`
	TeamSlug            slug.Slug                       `json:"-"`
	WorkloadReference   *workload.Reference             `json:"-"`
}

func (SQLInstance) IsPersistence() {}
func (SQLInstance) IsSearchNode()  {}
func (SQLInstance) IsNode()        {}

func (i *SQLInstance) GetName() string { return i.Name }

func (i *SQLInstance) GetNamespace() string { return i.TeamSlug.String() }

func (i *SQLInstance) GetLabels() map[string]string { return nil }

func (i *SQLInstance) GetObjectKind() schema.ObjectKind {
	return schema.EmptyObjectKind
}

func (i *SQLInstance) DeepCopyObject() runtime.Object {
	return i
}

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

type SQLInstanceUser struct {
	Name           string `json:"name"`
	Authentication string `json:"authentication"`
}

type SQLInstanceUserOrder struct {
	Field     SQLInstanceUserOrderField `json:"field"`
	Direction modelv1.OrderDirection    `json:"direction"`
}

type SQLInstanceUserOrderField string

const (
	SQLInstanceUserOrderFieldName           SQLInstanceUserOrderField = "NAME"
	SQLInstanceUserOrderFieldAuthentication SQLInstanceUserOrderField = "AUTHENTICATION"
)

func (e SQLInstanceUserOrderField) IsValid() bool {
	switch e {
	case SQLInstanceUserOrderFieldName, SQLInstanceUserOrderFieldAuthentication:
		return true
	}
	return false
}

func (e SQLInstanceUserOrderField) String() string {
	return string(e)
}

func (e *SQLInstanceUserOrderField) UnmarshalGQL(v interface{}) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = SQLInstanceUserOrderField(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid SqlInstanceUserOrderField", str)
	}
	return nil
}

func (e SQLInstanceUserOrderField) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

type SQLInstanceOrder struct {
	Field     SQLInstanceOrderField  `json:"field"`
	Direction modelv1.OrderDirection `json:"direction"`
}

type SQLInstanceOrderField string

const (
	SQLInstanceOrderFieldName        SQLInstanceOrderField = "NAME"
	SQLInstanceOrderFieldVersion     SQLInstanceOrderField = "VERSION"
	SQLInstanceOrderFieldEnvironment SQLInstanceOrderField = "ENVIRONMENT"
	SQLInstanceOrderFieldStatus      SQLInstanceOrderField = "STATUS"
	SQLInstanceOrderFieldCost        SQLInstanceOrderField = "COST"
	SQLInstanceOrderFieldCPU         SQLInstanceOrderField = "CPU_UTILIZATION"
	SQLInstanceOrderFieldMemory      SQLInstanceOrderField = "MEMORY_UTILIZATION"
	SQLInstanceOrderFieldDisk        SQLInstanceOrderField = "DISK_UTILIZATION"
)

func (e SQLInstanceOrderField) IsValid() bool {
	switch e {
	case SQLInstanceOrderFieldName, SQLInstanceOrderFieldVersion, SQLInstanceOrderFieldEnvironment, SQLInstanceOrderFieldStatus, SQLInstanceOrderFieldCost, SQLInstanceOrderFieldCPU, SQLInstanceOrderFieldMemory, SQLInstanceOrderFieldDisk:
		return true
	}
	return false
}

func (e SQLInstanceOrderField) String() string {
	return string(e)
}

func (e *SQLInstanceOrderField) UnmarshalGQL(v interface{}) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = SQLInstanceOrderField(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid SqlInstanceOrderField", str)
	}
	return nil
}

func (e SQLInstanceOrderField) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
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

func toSQLInstanceStatus(status sql_cnrm_cloud_google_com_v1beta1.SQLInstanceStatus) *SQLInstanceStatus {
	ret := &SQLInstanceStatus{
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

func toSQLDatabase(u *unstructured.Unstructured, environmentName string) (*SQLDatabase, error) {
	obj := &sql_cnrm_cloud_google_com_v1beta1.SQLDatabase{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, obj); err != nil {
		return nil, fmt.Errorf("converting to SQL database: %w", err)
	}

	return &SQLDatabase{
		Name:            ptr.Deref(obj.Spec.ResourceID, obj.ObjectMeta.Name),
		Charset:         obj.Spec.Charset,
		Collation:       obj.Spec.Collation,
		DeletionPolicy:  obj.Spec.DeletionPolicy,
		Healthy:         healthy(obj.Status.Conditions),
		SQLInstanceName: obj.Spec.InstanceRef.Name,
		EnvironmentName: environmentName,
		TeamSlug:        slug.Slug(obj.GetNamespace()),
	}, nil
}

func toSQLInstance(u *unstructured.Unstructured, environmentName string) (*SQLInstance, error) {
	obj := &sql_cnrm_cloud_google_com_v1beta1.SQLInstance{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, obj); err != nil {
		return nil, fmt.Errorf("converting to SQL instance: %w", err)
	}

	projectID := obj.GetAnnotations()["cnrm.cloud.google.com/project-id"]
	if projectID == "" {
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
		ProjectID:           projectID,
		Tier:                obj.Spec.Settings.Tier,
		Version:             obj.Spec.DatabaseVersion,
		Status:              toSQLInstanceStatus(obj.Status),
		EnvironmentName:     environmentName,
		TeamSlug:            slug.Slug(obj.GetNamespace()),
		WorkloadReference:   workload.ReferenceFromOwnerReferences(obj.OwnerReferences),
		Flags:               toSQLInstanceFlags(obj.Spec.Settings.DatabaseFlags),
	}, nil
}

func toSQLInstanceUser(user *sqladmin.User) *SQLInstanceUser {
	return &SQLInstanceUser{
		Name: user.Name,
		Authentication: func(t string) string {
			switch t {
			case "":
				return "Built-in"
			default:
				return t
			}
		}(user.Type),
	}
}

type TeamInventoryCountSQLInstances struct {
	Total int
}

type SQLInstanceMetrics struct {
	InstanceName string `json:"-"`
	ProjectID    string `json:"-"`
}

type SQLInstanceCPU struct {
	Cores       float64 `json:"cores"`
	Utilization float64 `json:"utilization"`
}

type SQLInstanceDisk struct {
	QuotaBytes  int     `json:"quotaBytes"`
	Utilization float64 `json:"utilization"`
}

type SQLInstanceMemory struct {
	QuotaBytes  int     `json:"quotaBytes"`
	Utilization float64 `json:"utilization"`
}

type SQLInstanceState string

const (
	SQLInstanceStateUnspecified   SQLInstanceState = "UNSPECIFIED"
	SQLInstanceStateRunnable      SQLInstanceState = "RUNNABLE"
	SQLInstanceStateSuspended     SQLInstanceState = "SUSPENDED"
	SQLInstanceStatePendingDelete SQLInstanceState = "PENDING_DELETE"
	SQLInstanceStatePendingCreate SQLInstanceState = "PENDING_CREATE"
	SQLInstanceStateMaintenance   SQLInstanceState = "MAINTENANCE"
	SQLInstanceStateFailed        SQLInstanceState = "FAILED"
)

var AllSQLInstanceState = []SQLInstanceState{
	SQLInstanceStateUnspecified,
	SQLInstanceStateRunnable,
	SQLInstanceStateSuspended,
	SQLInstanceStatePendingDelete,
	SQLInstanceStatePendingCreate,
	SQLInstanceStateMaintenance,
	SQLInstanceStateFailed,
}

func (e SQLInstanceState) String() string {
	return string(e)
}
