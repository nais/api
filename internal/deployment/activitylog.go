package deployment

import (
	"fmt"

	"github.com/nais/api/internal/activitylog"
)

const (
	activityLogResourceTypeDeployKey activitylog.ActivityLogResourceType = "DEPLOY_KEY"
)

func init() {
	activitylog.RegisterTransformer(activityLogResourceTypeDeployKey, func(entry activitylog.GenericActivityLogEntry) (activitylog.ActivityLogEntry, error) {
		switch entry.Action {
		case activitylog.ActivityLogActionUpdated:
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
