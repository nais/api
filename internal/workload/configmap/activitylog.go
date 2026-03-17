package configmap

import (
	"fmt"

	"github.com/nais/api/internal/activitylog"
)

const (
	activityLogEntryResourceTypeConfig activitylog.ActivityLogEntryResourceType = "CONFIG"
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
		case activitylog.ActivityLogEntryActionUpdated:
			data, err := activitylog.UnmarshalData[ConfigUpdatedActivityLogEntryData](entry)
			if err != nil {
				return nil, fmt.Errorf("failed to unmarshal config updated activity log entry data: %w", err)
			}

			return ConfigUpdatedActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage("Updated config"),
				Data:                    data,
			}, nil
		default:
			return nil, fmt.Errorf("unsupported config activity log entry action: %q", entry.Action)
		}
	})

	activitylog.RegisterFilter("CONFIG_CREATED", activitylog.ActivityLogEntryActionCreated, activityLogEntryResourceTypeConfig)
	activitylog.RegisterFilter("CONFIG_UPDATED", activitylog.ActivityLogEntryActionUpdated, activityLogEntryResourceTypeConfig)
	activitylog.RegisterFilter("CONFIG_DELETED", activitylog.ActivityLogEntryActionDeleted, activityLogEntryResourceTypeConfig)
}

type ConfigCreatedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
}

type ConfigUpdatedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
	Data *ConfigUpdatedActivityLogEntryData `json:"data"`
}

type ConfigUpdatedActivityLogEntryData struct {
	UpdatedFields []*ConfigUpdatedActivityLogEntryDataUpdatedField `json:"updatedFields"`
}

type ConfigUpdatedActivityLogEntryDataUpdatedField struct {
	Field    string  `json:"field"`
	OldValue *string `json:"oldValue,omitempty"`
	NewValue *string `json:"newValue,omitempty"`
}

type ConfigDeletedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
}
