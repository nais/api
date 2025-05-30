package servicemaintenance

import (
	"fmt"

	"github.com/nais/api/internal/activitylog"
)

const (
	activityLogResourceTypeValkeyServiceMaintenance     activitylog.ActivityLogEntryResourceType = "VALKEY_MAINTENANCE"
	activityLogResourceTypeOpenSearchServiceMaintenance activitylog.ActivityLogEntryResourceType = "OPENSEARCH_MAINTENANCE"
	activityLogEntryActionStartServiceMaintenance       activitylog.ActivityLogEntryAction       = "STARTED"
)

func init() {
	activitylog.RegisterTransformer(activityLogResourceTypeValkeyServiceMaintenance, func(entry activitylog.GenericActivityLogEntry) (activitylog.ActivityLogEntry, error) {
		if entry.Action == activityLogEntryActionStartServiceMaintenance {
			return ServiceMaintenanceActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage("Started service maintenance"),
			}, nil
		}
		return nil, fmt.Errorf("unsupported maintenance activity log entry action: %q", entry.Action)
	})
	activitylog.RegisterTransformer(activityLogResourceTypeOpenSearchServiceMaintenance, func(entry activitylog.GenericActivityLogEntry) (activitylog.ActivityLogEntry, error) {
		if entry.Action == activityLogEntryActionStartServiceMaintenance {
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
