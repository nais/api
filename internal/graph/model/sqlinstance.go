package model

type SQLInstance struct {
	AutoBackupHour      int         `json:"autoBackupHour"`
	CascadingDelete     bool        `json:"cascadingDelete"`
	Collation           string      `json:"collation"`
	Databases           []*Database `json:"databases"`
	DiskAutoresize      bool        `json:"diskAutoresize"`
	DiskSize            int         `json:"diskSize"`
	DiskType            string      `json:"diskType"`
	Environment         string      `json:"environment"`
	Flags               []*Flag     `json:"flags"`
	HighAvailability    bool        `json:"highAvailability"`
	Insights            Insights    `json:"insights"`
	Maintenance         Maintenance `json:"maintenance"`
	Name                string      `json:"name"`
	PointInTimeRecovery bool        `json:"pointInTimeRecovery"`
	RetainedBackups     int         `json:"retainedBackups"`
	Tier                string      `json:"tier"`
	Type                string      `json:"type"`
}

func (SQLInstance) IsStorage()           {}
func (this SQLInstance) GetName() string { return this.Name }
