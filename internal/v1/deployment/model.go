package deployment

import (
	"time"

	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/thirdparty/hookd"
	"github.com/nais/api/internal/v1/graphv1/ident"
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
