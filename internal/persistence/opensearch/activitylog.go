package opensearch

import (
	"fmt"

	"github.com/nais/api/internal/activitylog"
	servicemaintenanceal "github.com/nais/api/internal/servicemaintenance/activitylog"
)

const (
	ActivityLogEntryResourceTypeOpenSearch activitylog.ActivityLogEntryResourceType = "OPENSEARCH"
)

func init() {
	activitylog.RegisterTransformer(ActivityLogEntryResourceTypeOpenSearch, func(entry activitylog.GenericActivityLogEntry) (activitylog.ActivityLogEntry, error) {
		switch entry.Action {
		case activitylog.ActivityLogEntryActionCreated:
			return OpenSearchCreatedActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage("Created OpenSearch"),
			}, nil
		case activitylog.ActivityLogEntryActionUpdated:
			data, err := activitylog.UnmarshalData[OpenSearchUpdatedActivityLogEntryData](entry)
			if err != nil {
				return nil, fmt.Errorf("failed to unmarshal OpenSearch updated activity log entry data: %w", err)
			}
			return OpenSearchUpdatedActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage("Updated OpenSearch"),
				Data:                    data,
			}, nil
		case servicemaintenanceal.ActivityLogEntryActionMaintenanceStarted:
			return servicemaintenanceal.ServiceMaintenanceActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage("Started service maintenance"),
			}, nil
		default:
			return nil, fmt.Errorf("unsupported opensearch activity log entry action: %q", entry.Action)
		}
	})

	activitylog.RegisterFilter("OPENSEARCH_CREATED", activitylog.ActivityLogEntryActionCreated, ActivityLogEntryResourceTypeOpenSearch)
	activitylog.RegisterFilter("OPENSEARCH_UPDATED", activitylog.ActivityLogEntryActionUpdated, ActivityLogEntryResourceTypeOpenSearch)
}

type OpenSearchCreatedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
}

type OpenSearchUpdatedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
	Data *OpenSearchUpdatedActivityLogEntryData `json:"data"`
}

type OpenSearchUpdatedActivityLogEntryData struct {
	UpdatedFields []*OpenSearchUpdatedActivityLogEntryDataUpdatedField `json:"updatedFields"`
}

type OpenSearchUpdatedActivityLogEntryDataUpdatedField struct {
	// The name of the field.
	Field string `json:"field"`
	// The old value of the field.
	OldValue *string `json:"oldValue,omitempty"`
	// The new value of the field.
	NewValue *string `json:"newValue,omitempty"`
}
