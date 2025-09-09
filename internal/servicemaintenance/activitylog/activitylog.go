package activitylog

import (
	"github.com/nais/api/internal/activitylog"
)

const (
	ActivityLogEntryActionMaintenanceStarted activitylog.ActivityLogEntryAction = "MAINTENANCE_STARTED"
)

type ServiceMaintenanceActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
}
