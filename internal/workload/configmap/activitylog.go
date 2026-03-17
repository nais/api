package configmap

import (
	"fmt"

	"github.com/nais/api/internal/activitylog"
)

const (
	activityLogEntryResourceTypeConfig      activitylog.ActivityLogEntryResourceType = "CONFIG"
	activityLogEntryActionAddConfigValue    activitylog.ActivityLogEntryAction       = "ADD_CONFIG_VALUE"
	activityLogEntryActionUpdateConfigValue activitylog.ActivityLogEntryAction       = "UPDATE_CONFIG_VALUE"
	activityLogEntryActionRemoveConfigValue activitylog.ActivityLogEntryAction       = "REMOVE_CONFIG_VALUE"
)

func init() {
	activitylog.RegisterTransformer(activityLogEntryResourceTypeConfig, func(entry activitylog.GenericActivityLogEntry) (activitylog.ActivityLogEntry, error) {
		switch entry.Action {
		case activitylog.ActivityLogEntryActionCreated:
			return ConfigCreatedActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage("Created config"),
			}, nil
		case activitylog.ActivityLogEntryActionDeleted:
			return ConfigDeletedActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage("Deleted config"),
			}, nil
		case activityLogEntryActionAddConfigValue:
			data, err := activitylog.TransformData(entry, func(data *ConfigValueAddedActivityLogEntryData) *ConfigValueAddedActivityLogEntryData {
				return data
			})
			if err != nil {
				return nil, err
			}

			return ConfigValueAddedActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage("Added config value"),
				Data:                    data,
			}, nil
		case activityLogEntryActionUpdateConfigValue:
			data, err := activitylog.TransformData(entry, func(data *ConfigValueUpdatedActivityLogEntryData) *ConfigValueUpdatedActivityLogEntryData {
				return data
			})
			if err != nil {
				return nil, err
			}

			return ConfigValueUpdatedActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage("Updated config value"),
				Data:                    data,
			}, nil
		case activityLogEntryActionRemoveConfigValue:
			data, err := activitylog.TransformData(entry, func(data *ConfigValueRemovedActivityLogEntryData) *ConfigValueRemovedActivityLogEntryData {
				return data
			})
			if err != nil {
				return nil, err
			}

			return ConfigValueRemovedActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage("Removed config value"),
				Data:                    data,
			}, nil
		default:
			return nil, fmt.Errorf("unsupported config activity log entry action: %q", entry.Action)
		}
	})

	activitylog.RegisterFilter("CONFIG_CREATED", activitylog.ActivityLogEntryActionCreated, activityLogEntryResourceTypeConfig)
	activitylog.RegisterFilter("CONFIG_DELETED", activitylog.ActivityLogEntryActionDeleted, activityLogEntryResourceTypeConfig)
	activitylog.RegisterFilter("CONFIG_VALUE_ADDED", activityLogEntryActionAddConfigValue, activityLogEntryResourceTypeConfig)
	activitylog.RegisterFilter("CONFIG_VALUE_UPDATED", activityLogEntryActionUpdateConfigValue, activityLogEntryResourceTypeConfig)
	activitylog.RegisterFilter("CONFIG_VALUE_REMOVED", activityLogEntryActionRemoveConfigValue, activityLogEntryResourceTypeConfig)
}

type ConfigCreatedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
}

type ConfigValueAddedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
	Data *ConfigValueAddedActivityLogEntryData
}

type ConfigValueAddedActivityLogEntryData struct {
	ValueName string
}

type ConfigValueUpdatedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
	Data *ConfigValueUpdatedActivityLogEntryData
}

type ConfigValueUpdatedActivityLogEntryData struct {
	ValueName string
}

type ConfigValueRemovedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
	Data *ConfigValueRemovedActivityLogEntryData
}

type ConfigValueRemovedActivityLogEntryData struct {
	ValueName string
}

type ConfigDeletedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
}
