package application

import (
	"fmt"

	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/deployment/deploymentactivity"
)

const (
	activityLogEntryResourceTypeApplication activitylog.ActivityLogEntryResourceType = "APP"

	activityLogEntryActionRestartApplication   activitylog.ActivityLogEntryAction = "RESTARTED"
	activityLogEntryActionAutoScaleApplication activitylog.ActivityLogEntryAction = "AUTOSCALE"
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
		case activityLogEntryActionAutoScaleApplication:
			if entry.TeamSlug == nil {
				return nil, fmt.Errorf("missing team slug for application restart activity log entry")
			}
			if entry.EnvironmentName == nil {
				return nil, fmt.Errorf("missing environment name for application restart activity log entry")
			}
			data, err := activitylog.UnmarshalData[ApplicationScaledActivityLogEntryData](entry)
			if err != nil {
				return nil, fmt.Errorf("transforming application scaled activity log entry data: %w", err)
			}
			return ApplicationScaledActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage("Application scaled"),
				Data:                    data,
			}, nil
		case deploymentactivity.ActivityLogEntryActionDeployment:
			data, err := activitylog.UnmarshalData[deploymentactivity.DeploymentActivityLogEntryData](entry)
			if err != nil {
				return nil, fmt.Errorf("transforming application scaled activity log entry data: %w", err)
			}
			return deploymentactivity.DeploymentActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage("Application deployed"),
				Data:                    data,
			}, nil
		default:
			return nil, fmt.Errorf("unsupported application activity log entry action: %q", entry.Action)
		}
	})

	activitylog.RegisterFilter("APPLICATION_DELETED", activitylog.ActivityLogEntryActionDeleted, activityLogEntryResourceTypeApplication)
	activitylog.RegisterFilter("APPLICATION_RESTARTED", activityLogEntryActionRestartApplication, activityLogEntryResourceTypeApplication)
	activitylog.RegisterFilter("APPLICATION_SCALED", activityLogEntryActionAutoScaleApplication, activityLogEntryResourceTypeApplication)
	activitylog.RegisterFilter("DEPLOYMENT", deploymentactivity.ActivityLogEntryActionDeployment, activityLogEntryResourceTypeApplication)
}

type ApplicationRestartedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
}

type ApplicationDeletedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
}

type ApplicationScaledActivityLogEntry struct {
	activitylog.GenericActivityLogEntry

	Data *ApplicationScaledActivityLogEntryData `json:"data"`
}

type ApplicationScaledActivityLogEntryData struct {
	NewSize   int              `json:"newSize,string"`
	Direction ScalingDirection `json:"direction"`
}
