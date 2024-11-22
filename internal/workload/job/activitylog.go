package job

import (
	"fmt"

	"github.com/nais/api/internal/activitylog"
)

const (
	activityLogEntryResourceTypeJob  activitylog.ActivityLogEntryResourceType = "JOB"
	activityLogEntryActionTriggerJob activitylog.ActivityLogEntryAction       = "TRIGGER_JOB"
)

func init() {
	activitylog.RegisterTransformer(activityLogEntryResourceTypeJob, func(entry activitylog.GenericActivityLogEntry) (activitylog.ActivityLogEntry, error) {
		switch entry.Action {
		case activityLogEntryActionTriggerJob:
			return JobTriggeredActivityLog{
				GenericActivityLogEntry: entry.WithMessage("Job triggered"),
			}, nil
		case activitylog.ActivityLogEntryActionDeleted:
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
