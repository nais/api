package servicemaintenance

import (
	"fmt"

	"github.com/nais/api/internal/activitylog"
)

const (
	activityLogEntryResourceTypeVulnerability activitylog.ActivityLogEntryResourceType = "MAINTENANCE"
)

func init() {
	activitylog.RegisterTransformer(activityLogEntryResourceTypeVulnerability, func(entry activitylog.GenericActivityLogEntry) (activitylog.ActivityLogEntry, error) {
		switch entry.Action {
		case activitylog.ActivityLogEntryActionUpdated:
			return VulnerabilityUpdatedActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage("Updated maintenance"),
			}, nil
		default:
			return nil, fmt.Errorf("unsupported maintenance activity log entry action: %q", entry.Action)
		}
	})
}

type VulnerabilityUpdatedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
}
