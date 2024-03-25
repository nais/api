package model

import (
	"github.com/nais/api/internal/graph/scalar"
)

type BackupConfiguration struct {
	Enabled         bool   `json:"enabled"`
	StartTime       string `json:"startTime"`
	RetainedBackups int    `json:"retainedBackups"`
}

type SQLInstance struct {
	AutoBackupHour      int                  `json:"autoBackupHour"`
	BackupConfiguration *BackupConfiguration `json:"backupConfiguration"`
	CascadingDelete     bool                 `json:"cascadingDelete"`
	Collation           string               `json:"collation"`
	ConnectionName      string               `json:"connectionName"`
	DiskAutoresize      bool                 `json:"diskAutoresize"`
	DiskSize            int                  `json:"diskSize"`
	DiskType            string               `json:"diskType"`
	Env                 Env                  `json:"env"`
	Flags               []*Flag              `json:"flags"`
	HighAvailability    bool                 `json:"highAvailability"`
	ID                  scalar.Ident         `json:"id"`
	Insights            Insights             `json:"insights"`
	Maintenance         Maintenance          `json:"maintenance"`
	Name                string               `json:"name"`
	PointInTimeRecovery bool                 `json:"pointInTimeRecovery"`
	ProjectID           string               `json:"projectId"`
	RetainedBackups     int                  `json:"retainedBackups"`
	Team                *Team                `json:"team"`
	Tier                string               `json:"tier"`
	Type                string               `json:"type"`
	Status              SQLInstanceStatus    `json:"status"`
	GQLVars             SQLInstanceGQLVars   `json:"-"`
}

type SQLInstanceGQLVars struct {
	Labels      map[string]string
	Annotations map[string]string
}

func (SQLInstance) IsStorage()        {}
func (i SQLInstance) GetName() string { return i.Name }
func (i *SQLInstance) IsHealthy() bool {
	for _, cond := range i.Status.Conditions {
		if cond.Type == "Ready" && cond.Reason == "UpToDate" {
			return true
		}
	}
	return false
}
