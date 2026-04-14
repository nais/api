package valkey

import (
	"fmt"

	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/persistence/aivencredentials"
	servicemaintenanceal "github.com/nais/api/internal/servicemaintenance/activitylog"
)

const (
	ActivityLogEntryResourceTypeValkey activitylog.ActivityLogEntryResourceType = "VALKEY"
)

func init() {
	activitylog.RegisterTransformer(ActivityLogEntryResourceTypeValkey, func(entry activitylog.GenericActivityLogEntry) (activitylog.ActivityLogEntry, error) {
		switch entry.Action {
		case activitylog.ActivityLogEntryActionCreated:
			return ValkeyCreatedActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage("Created Valkey"),
			}, nil

		case activitylog.ActivityLogEntryActionUpdated:
			data, err := activitylog.UnmarshalData[ValkeyUpdatedActivityLogEntryData](entry)
			if err != nil {
				return nil, fmt.Errorf("failed to unmarshal Valkey updated activity log entry data: %w", err)
			}
			return ValkeyUpdatedActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage("Updated Valkey"),
				Data:                    data,
			}, nil
		case activitylog.ActivityLogEntryActionDeleted:
			return ValkeyDeletedActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage("Deleted Valkey"),
			}, nil
		case servicemaintenanceal.ActivityLogEntryActionMaintenanceStarted:
			return servicemaintenanceal.ServiceMaintenanceActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage("Started service maintenance"),
			}, nil
		case aivencredentials.ActivityLogEntryActionCredentialsCreated:
			return aivencredentials.GetActivityLogEntry(entry)
		default:
			return nil, fmt.Errorf("unsupported valkey activity log entry action: %q", entry.Action)
		}
	})

	activitylog.RegisterFilter("VALKEY_CREATED", activitylog.ActivityLogEntryActionCreated, ActivityLogEntryResourceTypeValkey)
	activitylog.RegisterFilter("VALKEY_UPDATED", activitylog.ActivityLogEntryActionUpdated, ActivityLogEntryResourceTypeValkey)
	activitylog.RegisterFilter("VALKEY_DELETED", activitylog.ActivityLogEntryActionDeleted, ActivityLogEntryResourceTypeValkey)
	activitylog.RegisterFilter("VALKEY_MAINTENANCE_STARTED", servicemaintenanceal.ActivityLogEntryActionMaintenanceStarted, ActivityLogEntryResourceTypeValkey)
	activitylog.RegisterFilter(aivencredentials.ActivityLogActivityTypeCredentialsCreated, aivencredentials.ActivityLogEntryActionCredentialsCreated, ActivityLogEntryResourceTypeValkey)
}

type ValkeyCreatedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
}

type ValkeyUpdatedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
	Data *ValkeyUpdatedActivityLogEntryData `json:"data"`
}

type ValkeyUpdatedActivityLogEntryData struct {
	UpdatedFields []*ValkeyUpdatedActivityLogEntryDataUpdatedField `json:"updatedFields"`
}

type ValkeyUpdatedActivityLogEntryDataUpdatedField struct {
	Field    string  `json:"field"`
	OldValue *string `json:"oldValue,omitempty"`
	NewValue *string `json:"newValue,omitempty"`
}

type ValkeyDeletedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
}
