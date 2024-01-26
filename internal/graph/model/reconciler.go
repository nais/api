package model

import (
	"github.com/google/uuid"
)

// Reconciler type.
type Reconciler struct {
	// The name of the reconciler.
	Name string `json:"name"`
	// The human-friendly name of the reconciler.
	DisplayName string `json:"displayName"`
	// Description of what the reconciler is responsible for.
	Description string `json:"description"`
	// Whether or not the reconciler is enabled.
	Enabled bool `json:"enabled"`
	// The run order of the reconciler.
	RunOrder int `json:"runOrder"`
}

type ReconcilerResource struct {
	// ID of the resource.
	ID uuid.UUID `json:"id"`
	// The name of the reconciler.
	Reconciler string `json:"reconciler"`
	// Key of the reconciler resource.
	Key string `json:"key"`
	// Value of the reconciler resource.
	Value string `json:"value"`
	// Metadata if any. JSON formatted.
	Metadata *string `json:"metadata"`
}
