package model

type Reconciler struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
	MemberAware bool   `json:"memberAware"`
}
