package department

import (
	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/slug"
)

type CreateDepartmentInput struct {
	// Unique team slug.
	//
	// After creation, this value can not be changed. Also, after a potential deletion of the team, the slug can not be
	// reused, so please choose wisely.
	Slug slug.Slug `json:"slug"`
	// The purpose / description of the team.
	//
	// What is the team for? What is the team working on? This value is meant for human consumption, and should be enough
	// to give a newcomer an idea of what the team is about.
	Purpose string `json:"purpose"`
	// The main Slack channel for the team.
	//
	// Where does the team communicate? This value is used to link to the team's main Slack channel.
	SlackChannel string `json:"slackChannel"`
}

type CreateDepartmentPayload struct {
	// The newly created team.
	Team *Department `json:"team,omitempty"`
}

// The team type represents a team on the [NAIS platform](https://nais.io/).
//
// Learn more about what NAIS departments are and what they can be used for in the [official NAIS documentation](https://docs.nais.io/explanations/team/).
//
// External resources (e.g. entraIDGroupID, gitHubdepartmentslug) are managed by [NAIS API reconcilers](https://github.com/nais/api-reconcilers).
type Department struct {
	// The globally unique ID of the team.
	ID ident.Ident `json:"id"`
	// Unique slug of the team.
	Slug slug.Slug `json:"slug"`
}

func (Department) IsNode() {}

type DepartmentConnection struct {
	// Pagination information.
	PageInfo *pagination.PageInfo `json:"pageInfo"`
	// List of nodes.
	Nodes []*Department `json:"nodes"`
	// List of edges.
	Edges []*DepartmentEdge `json:"edges"`
}

type DepartmentEdge struct {
	// Cursor for this edge that can be used for pagination.
	Cursor pagination.Cursor `json:"cursor"`
	// The team.
	Node *Department `json:"node"`
}

type DepartmentEnvironment struct {
	// The globally unique ID of the team environment.
	ID ident.Ident `json:"id"`
	// Name of the team environment.
	Name string `json:"name"`
	// The GCP project ID for the team environment.
	GCPProjectID *string `json:"gcpProjectID,omitempty"`
	// The Slack alerts channel for the team environment.
	SlackAlertsChannel string `json:"slackAlertsChannel"`
	// The connected team.
	Team *Department `json:"team"`
}

func (DepartmentEnvironment) IsNode() {}
