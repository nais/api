package apply

import (
	"fmt"

	"github.com/nais/api/internal/activitylog"
)

const (
	// ActivityLogEntryResourceTypeApply is the resource type for apply activity log entries.
	ActivityLogEntryResourceTypeApply activitylog.ActivityLogEntryResourceType = "APPLY"

	// ActivityLogEntryActionApplied is the action for a resource that was updated via apply.
	ActivityLogEntryActionApplied activitylog.ActivityLogEntryAction = "APPLIED"

	// ActivityLogEntryActionCreated is the action for a resource that was created via apply.
	ActivityLogEntryActionCreated activitylog.ActivityLogEntryAction = "APPLIED_NEW"
)

func init() {
	activitylog.RegisterTransformer(ActivityLogEntryResourceTypeApply, func(entry activitylog.GenericActivityLogEntry) (activitylog.ActivityLogEntry, error) {
		switch entry.Action {
		case ActivityLogEntryActionApplied:
			data, err := activitylog.UnmarshalData[ApplyActivityLogEntryData](entry)
			if err != nil {
				return nil, fmt.Errorf("unmarshaling apply activity log entry data: %w", err)
			}
			return ApplyActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage(fmt.Sprintf("Applied %s/%s", data.Kind, entry.ResourceName)),
				Data:                    data,
			}, nil
		case ActivityLogEntryActionCreated:
			data, err := activitylog.UnmarshalData[ApplyActivityLogEntryData](entry)
			if err != nil {
				return nil, fmt.Errorf("unmarshaling apply created activity log entry data: %w", err)
			}
			return ApplyActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage(fmt.Sprintf("Created %s/%s", data.Kind, entry.ResourceName)),
				Data:                    data,
			}, nil
		default:
			return nil, fmt.Errorf("unsupported apply activity log entry action: %q", entry.Action)
		}
	})

	activitylog.RegisterFilter("RESOURCE_APPLIED", ActivityLogEntryActionApplied, ActivityLogEntryResourceTypeApply)
	activitylog.RegisterFilter("RESOURCE_CREATED", ActivityLogEntryActionCreated, ActivityLogEntryResourceTypeApply)
}

// ApplyActivityLogEntryData contains the additional data stored with an apply activity log entry.
type ApplyActivityLogEntryData struct {
	// Cluster is the cluster the resource was applied to.
	Cluster string `json:"cluster"`

	// APIVersion is the apiVersion of the applied resource.
	APIVersion string `json:"apiVersion"`

	// Kind is the kind of the applied resource.
	Kind string `json:"kind"`

	// ChangedFields lists the fields that changed during the apply.
	ChangedFields []FieldChange `json:"changedFields"`
}

// ApplyActivityLogEntry is an activity log entry for an applied resource.
type ApplyActivityLogEntry struct {
	activitylog.GenericActivityLogEntry

	Data *ApplyActivityLogEntryData `json:"data"`
}
