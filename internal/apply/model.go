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

	// Cluster is the target cluster the resource was applied to.
	Cluster string `json:"cluster"`

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
