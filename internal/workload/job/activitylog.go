package job

import (
	"fmt"

	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/deployment/deploymentactivity"
)

const (
	ActivityLogEntryResourceTypeJob    activitylog.ActivityLogEntryResourceType = "JOB"
	activityLogEntryActionTriggerJob   activitylog.ActivityLogEntryAction       = "TRIGGER_JOB"
	activityLogEntryActionDeleteJobRun activitylog.ActivityLogEntryAction       = "DELETE_JOB_RUN"
)

func init() {
	activitylog.RegisterKindResourceType("Naisjob", ActivityLogEntryResourceTypeJob)
	activitylog.RegisterTransformer(ActivityLogEntryResourceTypeJob, func(entry activitylog.GenericActivityLogEntry) (activitylog.ActivityLogEntry, error) {
		switch entry.Action {
		case activityLogEntryActionTriggerJob:
			return JobTriggeredActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage("Job triggered"),
			}, nil
		case activitylog.ActivityLogEntryActionDeleted:
			return JobDeletedActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage("Job deleted"),
			}, nil

		case activityLogEntryActionDeleteJobRun:
			var data *JobRunDeletedActivityLogEntryData
			if entry.Data != nil {
				var err error
				data, err = activitylog.UnmarshalData[JobRunDeletedActivityLogEntryData](entry)
				if err != nil {
					return nil, fmt.Errorf("transforming job run deleted activity log entry data: %w", err)
				}
			}
			runName := entry.ResourceName
			if data != nil && data.RunName != "" {
				runName = data.RunName
			}
			return JobRunDeletedActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage(fmt.Sprintf("Job run %s deleted", runName)),
				Data:                    data,
			}, nil

		case deploymentactivity.ActivityLogEntryActionDeployment:
			data, err := activitylog.UnmarshalData[deploymentactivity.DeploymentActivityLogEntryData](entry)
			if err != nil {
				return nil, fmt.Errorf("transforming job deployment activity log entry data: %w", err)
			}
			return deploymentactivity.DeploymentActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage("Job deployed"),
				Data:                    data,
			}, nil
		case activitylog.ActivityLogEntryActionCreated:
			data, err := activitylog.UnmarshalData[activitylog.ResourceActivityLogEntryData](entry)
			if err != nil {
				return nil, fmt.Errorf("transforming job created activity log entry data: %w", err)
			}
			return JobCreatedActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage(fmt.Sprintf("Job %s created", entry.ResourceName)),
				Data:                    data,
			}, nil
		case activitylog.ActivityLogEntryActionUpdated:
			data, err := activitylog.UnmarshalData[activitylog.ResourceActivityLogEntryData](entry)
			if err != nil {
				return nil, fmt.Errorf("transforming job updated activity log entry data: %w", err)
			}
			return JobUpdatedActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage(fmt.Sprintf("Job %s updated", entry.ResourceName)),
				Data:                    data,
			}, nil
		default:
			return nil, fmt.Errorf("unsupported job activity log entry action: %q", entry.Action)
		}
	})

	activitylog.RegisterFilter("JOB_DELETED", activitylog.ActivityLogEntryActionDeleted, ActivityLogEntryResourceTypeJob)
	activitylog.RegisterFilter("JOB_RUN_DELETED", activityLogEntryActionDeleteJobRun, ActivityLogEntryResourceTypeJob)
	activitylog.RegisterFilter("JOB_TRIGGERED", activityLogEntryActionTriggerJob, ActivityLogEntryResourceTypeJob)
	activitylog.RegisterFilter("DEPLOYMENT", deploymentactivity.ActivityLogEntryActionDeployment, ActivityLogEntryResourceTypeJob)
	activitylog.RegisterFilter("RESOURCE_CREATED", activitylog.ActivityLogEntryActionCreated, ActivityLogEntryResourceTypeJob)
	activitylog.RegisterFilter("RESOURCE_UPDATED", activitylog.ActivityLogEntryActionUpdated, ActivityLogEntryResourceTypeJob)
}

type JobTriggeredActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
}

type JobDeletedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
}

type JobRunDeletedActivityLogEntryData struct {
	RunName string
}

type JobRunDeletedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
	Data *JobRunDeletedActivityLogEntryData
}

type JobCreatedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry

	Data *activitylog.ResourceActivityLogEntryData `json:"data"`
}

type JobUpdatedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry

	Data *activitylog.ResourceActivityLogEntryData `json:"data"`
}
