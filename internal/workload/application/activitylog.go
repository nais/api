package application

import (
	"fmt"

	"github.com/nais/api/internal/activitylog"
)

const (
	activityLogResourceTypeApplication  activitylog.ActivityLogResourceType = "APP"
	activityLogActionRestartApplication activitylog.ActivityLogAction       = "RESTARTED"
)

func init() {
	activitylog.RegisterTransformer(activityLogResourceTypeApplication, func(entry activitylog.GenericActivityLogEntry) (activitylog.ActivityLogEntry, error) {
		switch entry.Action {
		case activityLogActionRestartApplication:
			if entry.TeamSlug == nil {
				return nil, fmt.Errorf("missing team slug for application restart activity log entry")
			}
			if entry.EnvironmentName == nil {
				return nil, fmt.Errorf("missing environment name for application restart activity log entry")
			}
			return ApplicationRestartedActivityLog{
				GenericActivityLogEntry: entry.WithMessage("Application restarted"),
			}, nil
		case activitylog.ActivityLogActionDeleted:
			return ApplicationDeletedActivityLog{
				GenericActivityLogEntry: entry.WithMessage("Application deleted"),
			}, nil

		default:
			return nil, fmt.Errorf("unsupported application activity log entry action: %q", entry.Action)
		}
	})
}

type ApplicationRestartedActivityLog struct {
	activitylog.GenericActivityLogEntry
}

type ApplicationDeletedActivityLog struct {
	activitylog.GenericActivityLogEntry
}
