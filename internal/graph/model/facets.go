package model

type EnvironmentFacetItem struct {
	EnvironmentName string `json:"environmentName"`
	Count           int    `json:"count"`
}

type BooleanFacetItem struct {
	Value bool `json:"value"`
	Count int  `json:"count"`
}
