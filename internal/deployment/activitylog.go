package deployment

import (
	"fmt"

	"github.com/nais/api/internal/activitylog"
)

const (
	activityLogEntryResourceTypeDeployKey activitylog.ActivityLogEntryResourceType = "DEPLOY_KEY"
)

func init() {
	activitylog.RegisterTransformer(activityLogEntryResourceTypeDeployKey, func(entry activitylog.GenericActivityLogEntry) (activitylog.ActivityLogEntry, error) {
		switch entry.Action {
		case activitylog.ActivityLogEntryActionUpdated:
			return TeamDeployKeyUpdatedActivityLog{
				GenericActivityLogEntry: entry.WithMessage("Updated deployment key"),
			}, nil
		default:
			return nil, fmt.Errorf("unsupported deploy key activity log entry action: %q", entry.Action)
		}
	})
}

type TeamDeployKeyUpdatedActivityLog struct {
	activitylog.GenericActivityLogEntry
}
