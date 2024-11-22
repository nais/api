package secret

import (
	"fmt"

	"github.com/nais/api/internal/activitylog"
)

const (
	activityLogEntryResourceTypeSecret      activitylog.ActivityLogEntryResourceType = "SECRET"
	activityLogEntryActionAddSecretValue    activitylog.ActivityLogEntryAction       = "ADD_SECRET_VALUE"
	activityLogEntryActionUpdateSecretValue                                          = "UPDATE_SECRET_VALUE"
	activityLogEntryActionRemoveSecretValue                                          = "REMOVE_SECRET_VALUE"
)

func init() {
	activitylog.RegisterTransformer(activityLogEntryResourceTypeSecret, func(entry activitylog.GenericActivityLogEntry) (activitylog.ActivityLogEntry, error) {
		switch entry.Action {
		case activitylog.ActivityLogEntryActionCreated:
			return SecretCreatedActivityLog{
				GenericActivityLogEntry: entry.WithMessage("Created secret"),
			}, nil
		case activitylog.ActivityLogEntryActionDeleted:
			return SecretDeletedActivityLog{
				GenericActivityLogEntry: entry.WithMessage("Deleted secret"),
			}, nil
		case activityLogEntryActionAddSecretValue:
			data, err := activitylog.TransformData(entry, func(data *SecretValueAddedActivityLogData) *SecretValueAddedActivityLogData {
				return data
			})
			if err != nil {
				return nil, err
			}

			return SecretValueAddedActivityLog{
				GenericActivityLogEntry: entry.WithMessage("Added secret value"),
				Data:                    data,
			}, nil
		case activityLogEntryActionUpdateSecretValue:
			data, err := activitylog.TransformData(entry, func(data *SecretValueUpdatedActivityLogData) *SecretValueUpdatedActivityLogData {
				return data
			})
			if err != nil {
				return nil, err
			}

			return SecretValueUpdatedActivityLog{
				GenericActivityLogEntry: entry.WithMessage("Updated secret value"),
				Data:                    data,
			}, nil
		case activityLogEntryActionRemoveSecretValue:
			data, err := activitylog.TransformData(entry, func(data *SecretValueRemovedActivityLogData) *SecretValueRemovedActivityLogData {
				return data
			})
			if err != nil {
				return nil, err
			}

			return SecretValueRemovedActivityLog{
				GenericActivityLogEntry: entry.WithMessage("Removed secret value"),
				Data:                    data,
			}, nil
		default:
			return nil, fmt.Errorf("unsupported secret activity log entry action: %q", entry.Action)
		}
	})
}

type SecretCreatedActivityLog struct {
	activitylog.GenericActivityLogEntry
}

type SecretValueAddedActivityLog struct {
	activitylog.GenericActivityLogEntry
	Data *SecretValueAddedActivityLogData
}

type SecretValueAddedActivityLogData struct {
	ValueName string
}

type SecretValueUpdatedActivityLog struct {
	activitylog.GenericActivityLogEntry
	Data *SecretValueUpdatedActivityLogData
}

type SecretValueUpdatedActivityLogData struct {
	ValueName string
}

type SecretValueRemovedActivityLog struct {
	activitylog.GenericActivityLogEntry
	Data *SecretValueRemovedActivityLogData
}

type SecretValueRemovedActivityLogData struct {
	ValueName string
}

type SecretDeletedActivityLog struct {
	activitylog.GenericActivityLogEntry
}
