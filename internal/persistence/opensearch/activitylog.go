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
		case activitylog.ActivityLogEntryActionDeleted:
			return OpenSearchDeletedActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage("Deleted OpenSearch"),
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
	activitylog.RegisterFilter("OPENSEARCH_DELETED", activitylog.ActivityLogEntryActionDeleted, ActivityLogEntryResourceTypeOpenSearch)
	activitylog.RegisterFilter("OPENSEARCH_MAINTENANCE_STARTED", servicemaintenanceal.ActivityLogEntryActionMaintenanceStarted, ActivityLogEntryResourceTypeOpenSearch)
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
	Field    string  `json:"field"`
	OldValue *string `json:"oldValue,omitempty"`
	NewValue *string `json:"newValue,omitempty"`
}

type OpenSearchDeletedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
}
