package deploymentactivity

import (
	"fmt"

	"github.com/nais/api/internal/activitylog"
)

const (
	ActivityLogEntryResourceTypeDeployKey activitylog.ActivityLogEntryResourceType = "DEPLOY_KEY"
	ActivityLogEntryActionDeployment      activitylog.ActivityLogEntryAction       = "DEPLOYMENT"
)

func init() {
	activitylog.RegisterTransformer(ActivityLogEntryResourceTypeDeployKey, func(entry activitylog.GenericActivityLogEntry) (activitylog.ActivityLogEntry, error) {
		switch entry.Action {
		case activitylog.ActivityLogEntryActionUpdated:
			return TeamDeployKeyUpdatedActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage("Updated deployment key"),
			}, nil
		default:
			return nil, fmt.Errorf("unsupported deploy key activity log entry action: %q", entry.Action)
		}
	})
}

type TeamDeployKeyUpdatedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
}

type DeploymentActivityLogEntry struct {
	activitylog.GenericActivityLogEntry

	Data *DeploymentActivityLogEntryData `json:"data"`
}

type DeploymentActivityLogEntryData struct {
	TriggerURL string `json:"triggerURL,omitempty"`
}
