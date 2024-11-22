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
			return JobTriggeredActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage("Job triggered"),
			}, nil
		case activitylog.ActivityLogEntryActionDeleted:
			return JobDeletedActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage("Job deleted"),
			}, nil
		default:
			return nil, fmt.Errorf("unsupported job activity log entry action: %q", entry.Action)
		}
	})
}

type JobTriggeredActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
}

type JobDeletedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
}
