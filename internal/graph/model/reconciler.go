package model

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
	// Whether or not the reconciler uses team memberships when syncing.
	MemberAware bool `json:"memberAware"`
}
