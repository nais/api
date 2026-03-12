package activitylog

import (
	"fmt"
	"sync"
)

var (
	kindResourceTypes   = map[string]ActivityLogEntryResourceType{}
	kindResourceTypesMu sync.RWMutex
)

// RegisterKindResourceType registers a mapping from a Kubernetes kind string
// (e.g. "Application") to an ActivityLogEntryResourceType (e.g. "APP").
// Domain packages call this in their init() so that the apply handler can
// resolve the correct resource type without importing the domain package.
// Panics if the same kind is registered twice.
func RegisterKindResourceType(kind string, resourceType ActivityLogEntryResourceType) {
	kindResourceTypesMu.Lock()
	defer kindResourceTypesMu.Unlock()
	if _, ok := kindResourceTypes[kind]; ok {
		panic("kind resource type already registered: " + kind)
	}
	kindResourceTypes[kind] = resourceType
}

// ResourceTypeForKind returns the ActivityLogEntryResourceType for the given
// Kubernetes kind, falling back to the kind itself (uppercased) if no mapping
// has been registered. The bool is false only when the kind is empty.
func ResourceTypeForKind(kind string) (ActivityLogEntryResourceType, bool) {
	if kind == "" {
		return "", false
	}
	kindResourceTypesMu.RLock()
	rt, ok := kindResourceTypes[kind]
	kindResourceTypesMu.RUnlock()
	if ok {
		return rt, true
	}
	// Unknown kind — use the kind string itself as the resource type so that
	// the fallback transformer can handle it.
	return ActivityLogEntryResourceType(kind), true
}

// ResourceChangedField represents a single field that changed during a resource apply operation.
type ResourceChangedField struct {
	// Field is the dot-separated path to the changed field, e.g. "spec.replicas".
	Field string `json:"field"`

	// OldValue is the string representation of the value before the apply. Nil if the field was added.
	OldValue *string `json:"oldValue,omitempty"`

	// NewValue is the string representation of the value after the apply. Nil if the field was removed.
	NewValue *string `json:"newValue,omitempty"`
}

// ResourceActivityLogEntryData contains the additional data stored with a resource
// created or updated via apply.
type ResourceActivityLogEntryData struct {
	// APIVersion is the apiVersion of the applied resource.
	APIVersion string `json:"apiVersion"`

	// Kind is the kind of the applied resource.
	Kind string `json:"kind"`

	// ChangedFields lists the fields that changed during the apply.
	// Only populated for updates.
	ChangedFields []ResourceChangedField `json:"changedFields"`
}

// UnsupportedResourceActivityLogEntry is used for resource types that do not have
// a dedicated transformer registered — i.e. kinds that are not modelled in the
// GraphQL API. The uppercase Kind string is used directly as the resource type.
type UnsupportedResourceActivityLogEntry struct {
	GenericActivityLogEntry

	Data *ResourceActivityLogEntryData `json:"data"`
}

var fallbackTransformer Transformer

// RegisterFallbackTransformer registers a transformer that is called when no
// specific transformer has been registered for a resource type. Only one fallback
// may be registered; a second call panics.
func RegisterFallbackTransformer(t Transformer) {
	if fallbackTransformer != nil {
		panic("fallback transformer already registered")
	}
	fallbackTransformer = t
}

func init() {
	RegisterFallbackTransformer(func(entry GenericActivityLogEntry) (ActivityLogEntry, error) {
		data, err := UnmarshalData[ResourceActivityLogEntryData](entry)
		if err != nil {
			return nil, fmt.Errorf("transforming unsupported resource activity log entry data: %w", err)
		}
		return UnsupportedResourceActivityLogEntry{
			GenericActivityLogEntry: entry.WithMessage(
				fmt.Sprintf("%s %s %s", entry.ResourceName, entry.Action, entry.ResourceType),
			),
			Data: data,
		}, nil
	})
}
