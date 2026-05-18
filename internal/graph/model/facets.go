package model

// EnvironmentFacetItem is a shared facet item for environment distribution.
// Used across multiple resource types.
type EnvironmentFacetItem struct {
	EnvironmentName string `json:"environmentName"`
	Count           int    `json:"count"`
}

// BooleanFacetItem is a shared facet item for boolean distributions
// (e.g., "in use" or "high availability").
type BooleanFacetItem struct {
	Value bool `json:"value"`
	Count int  `json:"count"`
}
