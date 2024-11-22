package application

import (
	"fmt"

	"github.com/nais/api/internal/activitylog"
)

const (
	activityLogEntryResourceTypeApplication  activitylog.ActivityLogEntryResourceType = "APP"
	activityLogEntryActionRestartApplication activitylog.ActivityLogEntryAction       = "RESTARTED"
)

func init() {
	activitylog.RegisterTransformer(activityLogEntryResourceTypeApplication, func(entry activitylog.GenericActivityLogEntry) (activitylog.ActivityLogEntry, error) {
		switch entry.Action {
		case activityLogEntryActionRestartApplication:
			if entry.TeamSlug == nil {
				return nil, fmt.Errorf("missing team slug for application restart activity log entry")
			}
			if entry.EnvironmentName == nil {
				return nil, fmt.Errorf("missing environment name for application restart activity log entry")
			}
			return ApplicationRestartedActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage("Application restarted"),
			}, nil
		case activitylog.ActivityLogEntryActionDeleted:
			return ApplicationDeletedActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage("Application deleted"),
			}, nil

		default:
			return nil, fmt.Errorf("unsupported application activity log entry action: %q", entry.Action)
		}
	})
}

type ApplicationRestartedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
}

type ApplicationDeletedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
}
