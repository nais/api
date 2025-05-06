package maintenance

import (
	"fmt"

	"github.com/nais/api/internal/activitylog"
)

const (
	activityLogEntryResourceTypeVulnerability activitylog.ActivityLogEntryResourceType = "VULNERABILITY"
)

func init() {
	activitylog.RegisterTransformer(activityLogEntryResourceTypeVulnerability, func(entry activitylog.GenericActivityLogEntry) (activitylog.ActivityLogEntry, error) {
		switch entry.Action {
		case activitylog.ActivityLogEntryActionUpdated:
			return VulnerabilityUpdatedActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage("Updated vulnerability"),
			}, nil
		default:
			return nil, fmt.Errorf("unsupported vulnerability activity log entry action: %q", entry.Action)
		}
	})
}

type VulnerabilityUpdatedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
}
