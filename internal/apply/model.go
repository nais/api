package apply

// Response is the top-level response returned by the apply endpoint.
type Response struct {
	Results []ResourceResult `json:"results"`
}

// ResourceResult represents the outcome of applying a single resource.
type ResourceResult struct {
	// Resource is a human-readable identifier for the resource, e.g. "Application/my-app".
	Resource string `json:"resource"`

	// Namespace is the target namespace (== team slug) of the resource.
	Namespace string `json:"namespace"`

	// Environment is the target environment the resource was applied to.
	Environment string `json:"environment"`

	// Status is one of "created", "applied", or "error".
	Status string `json:"status"`

	// ChangedFields lists the fields that were changed during the apply.
	// Only populated when Status is "applied" (i.e. an update, not a create).
	ChangedFields []FieldChange `json:"changedFields,omitempty"`

	// Error contains the error message if Status is "error".
	Error string `json:"error,omitempty"`
}

const (
	StatusCreated = "created"
	StatusApplied = "applied"
	StatusError   = "error"
)

// ApplyChangedField is the GraphQL model for a single field that changed during an apply operation.
// This is the canonical type used by both the resolver and the GraphQL schema.
type ApplyChangedField struct {
	// Field is the dot-separated path to the changed field, e.g. "spec.replicas".
	Field string `json:"field"`
	// OldValue is the value before the apply. Nil if the field was added.
	OldValue *string `json:"oldValue,omitempty"`
	// NewValue is the value after the apply. Nil if the field was removed.
	NewValue *string `json:"newValue,omitempty"`
}
