package model

import (
	"github.com/nais/api/internal/graph/scalar"
	"github.com/nais/api/internal/slug"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type BackupConfiguration struct {
	Enabled             bool   `json:"enabled"`
	StartTime           string `json:"startTime"`
	RetainedBackups     int    `json:"retainedBackups"`
	PointInTimeRecovery bool   `json:"pointInTimeRecovery"`
}

type SQLInstance struct {
	BackupConfiguration *BackupConfiguration `json:"backupConfiguration"`
	CascadingDelete     bool                 `json:"cascadingDelete"`
	ConnectionName      string               `json:"connectionName"`
	Env                 Env                  `json:"env"`
	Flags               []*Flag              `json:"flags"`
	HighAvailability    bool                 `json:"highAvailability"`
	ID                  scalar.Ident         `json:"id"`
	MaintenanceWindow   *MaintenanceWindow   `json:"maintenanceWindow"`
	MaintenanceVersion  *string              `json:"maintenanceVersion"`
	Name                string               `json:"name"`
	ProjectID           string               `json:"projectId"`
	Tier                string               `json:"tier"`
	Type                string               `json:"type"`
	Status              SQLInstanceStatus    `json:"status"`
	GQLVars             SQLInstanceGQLVars   `json:"-"`
}

type SQLInstanceGQLVars struct {
	TeamSlug       slug.Slug
	Labels         map[string]string
	Annotations    map[string]string
	OwnerReference *v1.OwnerReference
}

func (SQLInstance) IsStorage() {}

func (i SQLInstance) GetName() string { return i.Name }

func (i *SQLInstance) IsHealthy() bool {
	for _, cond := range i.Status.Conditions {
		if cond.Type == "Ready" && cond.Reason == "UpToDate" && cond.Status == "True" {
			return true
		}
	}
	return false
}

type SQLInstanceMetrics struct {
	GQLVars SQLInstanceMetricsGQLVars `json:"-"`
}

type SQLInstanceMetricsGQLVars struct {
	DatabaseID string
	ProjectID  string
}
