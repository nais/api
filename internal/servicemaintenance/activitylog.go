package servicemaintenance

import (
	"fmt"

	"github.com/nais/api/internal/activitylog"
)

const (
	activityLogResourceTypeValkey            activitylog.ActivityLogEntryResourceType = "VALKEY"
	activityLogResourceTypeOpenSearch        activitylog.ActivityLogEntryResourceType = "OPENSEARCH"
	activityLogEntryActionMaintenanceStarted activitylog.ActivityLogEntryAction       = "MAINTENANCE_STARTED"
)

func init() {
	activitylog.RegisterTransformer(activityLogResourceTypeValkey, func(entry activitylog.GenericActivityLogEntry) (activitylog.ActivityLogEntry, error) {
		if entry.Action == activityLogEntryActionMaintenanceStarted {
			return ServiceMaintenanceActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage("Started service maintenance"),
			}, nil
		}
		return nil, fmt.Errorf("unsupported maintenance activity log entry action: %q", entry.Action)
	})
	activitylog.RegisterTransformer(activityLogResourceTypeOpenSearch, func(entry activitylog.GenericActivityLogEntry) (activitylog.ActivityLogEntry, error) {
		if entry.Action == activityLogEntryActionMaintenanceStarted {
			return ServiceMaintenanceActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage("Started service maintenance"),
			}, nil
		}
		return nil, fmt.Errorf("unsupported maintenance activity log entry action: %q", entry.Action)
	})
}

type ServiceMaintenanceActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
}
