package pubsublog

import (
	"fmt"

	"github.com/nais/api/internal/activitylog"
)

const (
	ActivityLogEntryResourceTypeClusterAudit activitylog.ActivityLogEntryResourceType = "CLUSTER_AUDIT"
	activityLogEntryActionClusterAudit       activitylog.ActivityLogEntryAction       = "CLUSTER_AUDIT"
	activityLogActivityTypeClusterAudit      activitylog.ActivityLogActivityType      = "CLUSTER_AUDIT"
)

func init() {
	activitylog.RegisterFilter(activityLogActivityTypeClusterAudit, activityLogEntryActionClusterAudit, ActivityLogEntryResourceTypeClusterAudit)

	activitylog.RegisterTransformer(ActivityLogEntryResourceTypeClusterAudit, func(entry activitylog.GenericActivityLogEntry) (activitylog.ActivityLogEntry, error) {
		data, err := activitylog.UnmarshalData[ClusterAuditActivityLogEntryData](entry)
		if err != nil {
			return nil, err
		}

		var msg string
		switch data.Action {
		case "Patch":
			msg = fmt.Sprintf("Patched %v %v", data.ResourceKind, entry.ResourceName)
		case "Create":
			msg = fmt.Sprintf("Created %v %v", data.ResourceKind, entry.ResourceName)
		case "Delete":
			msg = fmt.Sprintf("Deleted %v %v", data.ResourceKind, entry.ResourceName)
		default:
			msg = fmt.Sprintf("Performed %v on %v %v", data.Action, data.ResourceKind, entry.ResourceName)
		}

		return ClusterAuditActivityLogEntry{
			GenericActivityLogEntry: entry.WithMessage(msg),
			Data:                    data,
		}, nil
	})
}
