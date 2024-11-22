package secret

import (
	"fmt"

	"github.com/nais/api/internal/activitylog"
)

const (
	activityLogResourceTypeSecret      activitylog.ActivityLogResourceType = "SECRET"
	activityLogActionAddSecretValue    activitylog.ActivityLogAction       = "ADD_SECRET_VALUE"
	activityLogActionUpdateSecretValue                                     = "UPDATE_SECRET_VALUE"
	activityLogActionRemoveSecretValue                                     = "REMOVE_SECRET_VALUE"
)

func init() {
	activitylog.RegisterTransformer(activityLogResourceTypeSecret, func(entry activitylog.GenericActivityLogEntry) (activitylog.ActivityLogEntry, error) {
		switch entry.Action {
		case activitylog.ActivityLogActionCreated:
			return SecretCreatedActivityLog{
				GenericActivityLogEntry: entry.WithMessage("Created secret"),
			}, nil
		case activitylog.ActivityLogActionDeleted:
			return SecretDeletedActivityLog{
				GenericActivityLogEntry: entry.WithMessage("Deleted secret"),
			}, nil
		case activityLogActionAddSecretValue:
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
		case activityLogActionUpdateSecretValue:
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
		case activityLogActionRemoveSecretValue:
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
