package job

import (
	"fmt"

	"github.com/nais/api/internal/activitylog"
)

const (
	activityLogResourceTypeJob  activitylog.ActivityLogResourceType = "JOB"
	activityLogActionTriggerJob activitylog.ActivityLogAction       = "TRIGGER_JOB"
)

func init() {
	activitylog.RegisterTransformer(activityLogResourceTypeJob, func(entry activitylog.GenericActivityLogEntry) (activitylog.ActivityLogEntry, error) {
		switch entry.Action {
		case activityLogActionTriggerJob:
			return JobTriggeredActivityLog{
				GenericActivityLogEntry: entry.WithMessage("Job triggered"),
			}, nil
		case activitylog.ActivityLogActionDeleted:
			return JobDeletedActivityLog{
				GenericActivityLogEntry: entry.WithMessage("Job deleted"),
			}, nil
		default:
			return nil, fmt.Errorf("unsupported job activity log entry action: %q", entry.Action)
		}
	})
}

type JobTriggeredActivityLog struct {
	activitylog.GenericActivityLogEntry
}

type JobDeletedActivityLog struct {
	activitylog.GenericActivityLogEntry
}
