package model

import "github.com/nais/api/internal/graph/scalar"

type SQLInstance struct {
	AutoBackupHour      int               `json:"autoBackupHour"`
	CascadingDelete     bool              `json:"cascadingDelete"`
	Collation           string            `json:"collation"`
	Databases           []*Database       `json:"databases"`
	DiskAutoresize      bool              `json:"diskAutoresize"`
	DiskSize            int               `json:"diskSize"`
	DiskType            string            `json:"diskType"`
	Env                 Env               `json:"env"`
	Flags               []*Flag           `json:"flags"`
	HighAvailability    bool              `json:"highAvailability"`
	ID                  scalar.Ident      `json:"id"`
	Insights            Insights          `json:"insights"`
	Maintenance         Maintenance       `json:"maintenance"`
	Name                string            `json:"name"`
	PointInTimeRecovery bool              `json:"pointInTimeRecovery"`
	RetainedBackups     int               `json:"retainedBackups"`
	Tier                string            `json:"tier"`
	Type                string            `json:"type"`
	Status              SQLInstanceStatus `json:"status"`
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
