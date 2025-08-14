package pubsublog

import "github.com/nais/api/internal/activitylog"

type ClusterAuditActivityLogEntryData struct {
	Action       string `json:"action,omitempty"`
	ResourceKind string `json:"resourceKind,omitempty"`
}

type ClusterAuditActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
	Data *ClusterAuditActivityLogEntryData
}
