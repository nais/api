package model

type BooleanFacetItem struct {
	Value bool `json:"value"`
	Count int  `json:"count"`
}

type StringFacetItem struct {
	Value string `json:"value"`
	Count int    `json:"count"`
}
