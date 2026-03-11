package apply

import (
	"github.com/nais/api/internal/activitylog"
)

// ResourceTypeForKind returns the ActivityLogEntryResourceType for the given
// Kubernetes kind, so apply entries are stored under the correct resource type
// rather than a generic APPLY type.
func ResourceTypeForKind(kind string) (activitylog.ActivityLogEntryResourceType, bool) {
	switch kind {
	case "Application":
		return "APP", true
	case "Naisjob":
		return "JOB", true
	default:
		return "", false
	}
}

// ApplyActivityLogEntryData contains the additional data stored with a resource
// created or updated via apply.
type ApplyActivityLogEntryData struct {
	// APIVersion is the apiVersion of the applied resource.
	APIVersion string `json:"apiVersion"`

	// Kind is the kind of the applied resource.
	Kind string `json:"kind"`

	// ChangedFields lists the fields that changed during the apply.
	// Only populated for updates.
	ChangedFields []FieldChange `json:"changedFields"`
}
