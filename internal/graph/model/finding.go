package model

import "github.com/nais/api/internal/graph/scalar"

type Finding struct {
	ID              scalar.Ident   `json:"id"`
	ParentID        string         `json:"parentId"`
	VulnerabilityID string         `json:"vulnerabilityId"`
	VulnID          string         `json:"vulnId"`
	Source          string         `json:"source"`
	ComponentID     string         `json:"componentId"`
	Severity        string         `json:"severity"`
	Description     string         `json:"description"`
	PackageURL      string         `json:"packageUrl"`
	Aliases         []*VulnIDAlias `json:"aliases"`
	IsSuppressed    bool           `json:"isSuppressed"`
	State           string         `json:"state"`
}

type FindingList struct {
	Nodes    []*Finding `json:"nodes"`
	PageInfo PageInfo   `json:"pageInfo"`
}
