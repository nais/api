package model

import (
	"fmt"
	"strings"

	sql_cnrm_cloud_google_com_v1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/pkg/clients/generated/apis/sql/v1beta1"
	"github.com/nais/api/internal/graph/scalar"
	"github.com/nais/api/internal/slug"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

type BackupConfiguration struct {
	Enabled                     bool   `json:"enabled"`
	StartTime                   string `json:"startTime"`
	RetainedBackups             int    `json:"retainedBackups"`
	PointInTimeRecovery         bool   `json:"pointInTimeRecovery"`
	TransactionLogRetentionDays int    `json:"transactionLogRetentionDays"`
}

type SQLUser struct {
	Name           string `json:"name"`
	Authentication string `json:"authentication"`
}

type SQLInstance struct {
	BackupConfiguration *BackupConfiguration `json:"backupConfiguration"`
	CascadingDelete     bool                 `json:"cascadingDelete"`
	ConnectionName      string               `json:"connectionName"`
	DiskAutoresize      bool                 `json:"diskAutoresize"`
	DiskAutoresizeLimit int                  `json:"diskAutoresizeLimit"`
	Env                 Env                  `json:"env"`
	Flags               []*Flag              `json:"flags"`
	HighAvailability    bool                 `json:"highAvailability"`
	ID                  scalar.Ident         `json:"id"`
	MaintenanceWindow   *MaintenanceWindow   `json:"maintenanceWindow"`
	MaintenanceVersion  *string              `json:"maintenanceVersion"`
	Metrics             *SQLInstanceMetrics  `json:"metrics"`
	Name                string               `json:"name"`
	ProjectID           string               `json:"projectId"`
	SQLUsers            []*SQLUser           `json:"users"`
	Tier                string               `json:"tier"`
	Type                string               `json:"type"`
	State               SQLInstanceState     `json:"state"`
	Status              SQLInstanceStatus    `json:"status"`
	GQLVars             SQLInstanceGQLVars   `json:"-"`
}

type SQLInstanceGQLVars struct {
	TeamSlug       slug.Slug
	Labels         map[string]string
	Annotations    map[string]string
	OwnerReference *v1.OwnerReference
}

func (SQLInstance) IsPersistence() {}
func (SQLInstance) IsSearchNode()  {}

func (i SQLInstance) GetName() string { return i.Name }

func (i SQLInstance) IsHealthy() bool {
	for _, cond := range i.Status.Conditions {
		if cond.Type == string(corev1.PodReady) && cond.Reason == "UpToDate" && cond.Status == string(corev1.ConditionTrue) {
			return true
		}
	}
	return false
}

func (i SQLInstance) IsNotReady() bool {
	for _, cond := range i.Status.Conditions {
		if cond.Type != string(corev1.PodReady) {
			return true
		}
	}
	return false
}

type SQLInstanceMetrics struct {
	Cost   float64            `json:"cost"`
	CPU    *SQLInstanceCPU    `json:"cpu"`
	Memory *SQLInstanceMemory `json:"memory"`
	Disk   *SQLInstanceDisk   `json:"disk"`
}

type SQLInstancesList struct {
	Nodes    []*SQLInstance       `json:"nodes"`
	PageInfo PageInfo             `json:"pageInfo"`
	Metrics  *SQLInstancesMetrics `json:"metrics"`
}

func ToSqlInstance(u *unstructured.Unstructured, env string) (*SQLInstance, error) {
	sqlInstance := &sql_cnrm_cloud_google_com_v1beta1.SQLInstance{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, sqlInstance); err != nil {
		return nil, fmt.Errorf("converting to SQL instance: %w", err)
	}
	projectId := sqlInstance.GetAnnotations()["cnrm.cloud.google.com/project-id"]
	if projectId == "" {
		return nil, fmt.Errorf("missing project ID annotation")
	}

	teamSlug := sqlInstance.GetNamespace()

	return &SQLInstance{
		ID:   scalar.SqlInstanceIdent("sqlInstance_" + env + "_" + sqlInstance.GetNamespace() + "_" + sqlInstance.GetName()),
		Name: sqlInstance.Name,
		Env: Env{
			Name: env,
			Team: teamSlug,
		},
		BackupConfiguration: func(backupConfig *sql_cnrm_cloud_google_com_v1beta1.InstanceBackupConfiguration) *BackupConfiguration {
			if backupConfig == nil {
				return nil
			}
			backupCfg := &BackupConfiguration{}
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
			if backupConfig.TransactionLogRetentionDays != nil {
				backupCfg.TransactionLogRetentionDays = *backupConfig.TransactionLogRetentionDays
			}
			return backupCfg
		}(sqlInstance.Spec.Settings.BackupConfiguration),
		CascadingDelete: sqlInstance.GetAnnotations()["cnrm.cloud.google.com/deletion-policy"] != "abandon",
		Type:            *sqlInstance.Spec.DatabaseVersion,
		ConnectionName:  *sqlInstance.Status.ConnectionName,
		Status: SQLInstanceStatus{
			Conditions: func() []*Condition {
				ret := make([]*Condition, 0)
				for _, condition := range sqlInstance.Status.Conditions {
					ret = append(ret, &Condition{
						Type:               condition.Type,
						Status:             string(condition.Status),
						Reason:             condition.Reason,
						Message:            formatMessage(condition.Message),
						LastTransitionTime: condition.LastTransitionTime,
					})
				}
				return ret
			}(),
			PublicIPAddress: sqlInstance.Status.PublicIpAddress,
			PrivateIPAddress: func(status sql_cnrm_cloud_google_com_v1beta1.SQLInstanceStatus) *string {
				var privateIpAddresses *string
				for _, ip := range status.IpAddress {
					if *ip.Type == "PRIVATE" {
						privateIpAddresses = ip.IpAddress
					}
				}
				return privateIpAddresses
			}(sqlInstance.Status),
		},
		MaintenanceWindow: func(window *sql_cnrm_cloud_google_com_v1beta1.InstanceMaintenanceWindow) *MaintenanceWindow {
			if window == nil || window.Day == nil || window.Hour == nil {
				return nil
			}
			return &MaintenanceWindow{
				Day:  *window.Day,
				Hour: *window.Hour,
			}
		}(sqlInstance.Spec.Settings.MaintenanceWindow),
		Flags: func() []*Flag {
			ret := make([]*Flag, 0)
			for _, flag := range sqlInstance.Spec.Settings.DatabaseFlags {
				ret = append(ret, &Flag{
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
		DiskAutoresize:     *sqlInstance.Spec.Settings.DiskAutoresize,
		DiskAutoresizeLimit: func() int {
			if sqlInstance.Spec.Settings.DiskAutoresizeLimit == nil {
				return 0
			}
			return *sqlInstance.Spec.Settings.DiskAutoresizeLimit
		}(),
		GQLVars: SQLInstanceGQLVars{
			TeamSlug:       slug.Slug(teamSlug),
			Labels:         sqlInstance.GetLabels(),
			Annotations:    sqlInstance.GetAnnotations(),
			OwnerReference: OwnerReference(sqlInstance.OwnerReferences),
		},
	}, nil
}

func formatMessage(raw string) string {
	gapi := strings.SplitAfter(raw, "googleapi:")
	if len(gapi) > 1 {
		return strings.ReplaceAll(gapi[1], ",", "")
	}
	return raw
}

func equals(s *string, eq string) bool {
	return s != nil && *s == eq
}
