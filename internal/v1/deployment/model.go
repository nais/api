package deployment

import (
	"time"

	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/thirdparty/hookd"
	"github.com/nais/api/internal/v1/graphv1/ident"
	"github.com/nais/api/internal/v1/graphv1/pagination"
)

type (
	DeploymentConnection = pagination.Connection[*Deployment]
	DeploymentEdge       = pagination.Edge[*Deployment]
)

type ChangeDeploymentKeyInput struct {
	// The name of the team to update the deploy key for.
	TeamSlug slug.Slug `json:"team"`
}

type ChangeDeploymentKeyPayload struct {
	// The updated deploy key.
	DeploymentKey *DeploymentKey `json:"deploymentKey,omitempty"`
}

// Deployment key type.
type DeploymentKey struct {
	// The actual key.
	Key string `json:"key"`
	// The date the deployment key was created.
	Created time.Time `json:"created"`
	// The date the deployment key expires.
	Expires time.Time `json:"expires"`

	TeamSlug slug.Slug `json:"-"`
}

func toGraphDeploymentKey(d *hookd.DeployKey, teamSlug slug.Slug) *DeploymentKey {
	return &DeploymentKey{
		Key:      d.Key,
		Created:  d.Created,
		Expires:  d.Expires,
		TeamSlug: teamSlug,
	}
}

func (d DeploymentKey) ID() ident.Ident {
	return newIdent(d.TeamSlug)
}

func (DeploymentKey) IsNode() {}

type Deployment struct {
	Resources       []*DeploymentResource `json:"resources"`
	Statuses        []*DeploymentStatus   `json:"statuses"`
	Created         time.Time             `json:"created"`
	Repository      string                `json:"repository"`
	TeamSlug        slug.Slug             `json:"-"`
	EnvironmentName string                `json:"-"`
}

func toGraphDeployment(d hookd.Deploy) *Deployment {
	statuses := make([]*DeploymentStatus, len(d.Statuses))
	for i, s := range d.Statuses {
		var msg *string
		if s.Message != "" {
			msg = &s.Message
		}
		statuses[i] = &DeploymentStatus{
			Status:  s.Status,
			Message: msg,
			Created: s.Created,
		}
	}

	resources := make([]*DeploymentResource, len(d.Resources))
	for i, r := range d.Resources {
		resources[i] = &DeploymentResource{
			Group:     r.Group,
			Kind:      r.Kind,
			Name:      r.Name,
			Version:   r.Version,
			Namespace: r.Namespace,
		}
	}

	return &Deployment{
		Created:         d.DeploymentInfo.Created,
		Repository:      d.DeploymentInfo.GithubRepository,
		TeamSlug:        d.DeploymentInfo.Team,
		EnvironmentName: d.DeploymentInfo.Cluster,
	}
}

func (d Deployment) ID() ident.Ident {
	return newIdent(d.TeamSlug)
}

func (Deployment) IsNode() {}

type DeploymentResource struct {
	Group     string `json:"group"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Version   string `json:"version"`
	Namespace string `json:"namespace"`
}

type DeploymentStatus struct {
	Status  string    `json:"status"`
	Message *string   `json:"message,omitempty"`
	Created time.Time `json:"created"`
}
