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

// GenericKubernetesResourceActivityLogEntryData contains the additional data stored with a resource
// created or updated via apply.
type GenericKubernetesResourceActivityLogEntryData struct {
	// APIVersion is the apiVersion of the applied resource.
	APIVersion string `json:"apiVersion"`

	// Kind is the kind of the applied resource.
	Kind string `json:"kind"`

	// ChangedFields lists the fields that changed during the apply.
	// Only populated for updates.
	ChangedFields []ResourceChangedField `json:"changedFields"`

	// GitHubClaims holds the GitHub Actions OIDC token claims at the time of the
	// apply. Only populated when the request was authenticated via a GitHub token.
	GitHubClaims *GitHubActorClaims `json:"gitHubClaims,omitempty"`
}

// GitHubActorClaims holds the GitHub Actions OIDC token claims captured at the
// time of an apply operation. Duplicated from the middleware package to avoid a
// circular import; JSON tags must stay in sync.
type GitHubActorClaims struct {
	Ref            string `json:"ref"`
	Repository     string `json:"repository"`
	RepositoryID   string `json:"repositoryId"`
	RunID          string `json:"runId"`
	RunAttempt     string `json:"runAttempt"`
	Actor          string `json:"actor"`
	Workflow       string `json:"workflow"`
	EventName      string `json:"eventName"`
	Environment    string `json:"environment"`
	JobWorkflowRef string `json:"jobWorkflowRef"`
}

// GenericKubernetesActivityLogEntry is used for resource types that do not have
// a dedicated transformer registered — i.e. kinds that are not modelled in the
// GraphQL API. The uppercase Kind string is used directly as the resource type.
type GenericKubernetesResourceActivityLogEntry struct {
	GenericActivityLogEntry

	Data *GenericKubernetesResourceActivityLogEntryData `json:"data"`
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
		data, err := UnmarshalData[GenericKubernetesResourceActivityLogEntryData](entry)
		if err != nil {
			return nil, fmt.Errorf("transforming unsupported resource activity log entry data: %w", err)
		}
		return GenericKubernetesResourceActivityLogEntry{
			GenericActivityLogEntry: entry.WithMessage(
				fmt.Sprintf("%s %s %s", entry.ResourceName, entry.Action, entry.ResourceType),
			),
			Data: data,
		}, nil
	})
}
