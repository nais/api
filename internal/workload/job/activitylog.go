package job

import (
	"fmt"

	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/deployment/deploymentactivity"
)

const (
	activityLogEntryResourceTypeJob  activitylog.ActivityLogEntryResourceType = "JOB"
	activityLogEntryActionTriggerJob activitylog.ActivityLogEntryAction       = "TRIGGER_JOB"
)

func init() {
	activitylog.RegisterTransformer(activityLogEntryResourceTypeJob, func(entry activitylog.GenericActivityLogEntry) (activitylog.ActivityLogEntry, error) {
		switch entry.Action {
		case activityLogEntryActionTriggerJob:
			return JobTriggeredActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage("Job triggered"),
			}, nil
		case activitylog.ActivityLogEntryActionDeleted:
			return JobDeletedActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage("Job deleted"),
			}, nil

		case deploymentactivity.ActivityLogEntryActionDeployment:
			data, err := activitylog.UnmarshalData[deploymentactivity.DeploymentActivityLogEntryData](entry)
			if err != nil {
				return nil, fmt.Errorf("transforming job scaled activity log entry data: %w", err)
			}
			return deploymentactivity.DeploymentActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage("Job deployed"),
				Data:                    data,
			}, nil
		default:
			return nil, fmt.Errorf("unsupported job activity log entry action: %q", entry.Action)
		}
	})

	activitylog.RegisterFilter("JOB_DELETED", activitylog.ActivityLogEntryActionDeleted, activityLogEntryResourceTypeJob)
	activitylog.RegisterFilter("JOB_TRIGGERED", activityLogEntryActionTriggerJob, activityLogEntryResourceTypeJob)
	activitylog.RegisterFilter("DEPLOYMENT", deploymentactivity.ActivityLogEntryActionDeployment, activityLogEntryResourceTypeJob)
}

type JobTriggeredActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
}

type JobDeletedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
}
