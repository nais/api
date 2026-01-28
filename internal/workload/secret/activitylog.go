package secret

import (
	"fmt"

	"github.com/nais/api/internal/activitylog"
)

const (
	ActivityLogEntryResourceTypeSecret      activitylog.ActivityLogEntryResourceType = "SECRET"
	activityLogEntryActionAddSecretValue    activitylog.ActivityLogEntryAction       = "ADD_SECRET_VALUE"
	activityLogEntryActionUpdateSecretValue activitylog.ActivityLogEntryAction       = "UPDATE_SECRET_VALUE"
	activityLogEntryActionRemoveSecretValue activitylog.ActivityLogEntryAction       = "REMOVE_SECRET_VALUE"
)

func init() {
	activitylog.RegisterTransformer(ActivityLogEntryResourceTypeSecret, func(entry activitylog.GenericActivityLogEntry) (activitylog.ActivityLogEntry, error) {
		switch entry.Action {
		case activitylog.ActivityLogEntryActionCreated:
			return SecretCreatedActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage("Created secret"),
			}, nil
		case activitylog.ActivityLogEntryActionDeleted:
			return SecretDeletedActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage("Deleted secret"),
			}, nil
		case activityLogEntryActionAddSecretValue:
			data, err := activitylog.TransformData(entry, func(data *SecretValueAddedActivityLogEntryData) *SecretValueAddedActivityLogEntryData {
				return data
			})
			if err != nil {
				return nil, err
			}

			return SecretValueAddedActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage("Added secret value"),
				Data:                    data,
			}, nil
		case activityLogEntryActionUpdateSecretValue:
			data, err := activitylog.TransformData(entry, func(data *SecretValueUpdatedActivityLogEntryData) *SecretValueUpdatedActivityLogEntryData {
				return data
			})
			if err != nil {
				return nil, err
			}

			return SecretValueUpdatedActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage("Updated secret value"),
				Data:                    data,
			}, nil
		case activityLogEntryActionRemoveSecretValue:
			data, err := activitylog.TransformData(entry, func(data *SecretValueRemovedActivityLogEntryData) *SecretValueRemovedActivityLogEntryData {
				return data
			})
			if err != nil {
				return nil, err
			}

			return SecretValueRemovedActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage("Removed secret value"),
				Data:                    data,
			}, nil
		default:
			return nil, fmt.Errorf("unsupported secret activity log entry action: %q", entry.Action)
		}
	})

	activitylog.RegisterFilter("SECRET_CREATED", activitylog.ActivityLogEntryActionCreated, ActivityLogEntryResourceTypeSecret)
	activitylog.RegisterFilter("SECRET_DELETED", activitylog.ActivityLogEntryActionDeleted, ActivityLogEntryResourceTypeSecret)
	activitylog.RegisterFilter("SECRET_VALUE_ADDED", activityLogEntryActionAddSecretValue, ActivityLogEntryResourceTypeSecret)
	activitylog.RegisterFilter("SECRET_VALUE_UPDATED", activityLogEntryActionUpdateSecretValue, ActivityLogEntryResourceTypeSecret)
	activitylog.RegisterFilter("SECRET_VALUE_REMOVED", activityLogEntryActionRemoveSecretValue, ActivityLogEntryResourceTypeSecret)
}

type SecretCreatedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
}

type SecretValueAddedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
	Data *SecretValueAddedActivityLogEntryData
}

type SecretValueAddedActivityLogEntryData struct {
	ValueName string
}

type SecretValueUpdatedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
	Data *SecretValueUpdatedActivityLogEntryData
}

type SecretValueUpdatedActivityLogEntryData struct {
	ValueName string
}

type SecretValueRemovedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
	Data *SecretValueRemovedActivityLogEntryData
}

type SecretValueRemovedActivityLogEntryData struct {
	ValueName string
}

type SecretDeletedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
}
