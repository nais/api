package deployment

import (
	"time"

	"github.com/nais/api/internal/workload"

	"github.com/google/uuid"
	"github.com/nais/api/internal/deployment/deploymentsql"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/thirdparty/hookd"
)

type (
	DeploymentConnection         = pagination.Connection[*Deployment]
	DeploymentEdge               = pagination.Edge[*Deployment]
	DeploymentStatusConnection   = pagination.Connection[*DeploymentStatus]
	DeploymentStatusEdge         = pagination.Edge[*DeploymentStatus]
	DeploymentResourceConnection = pagination.Connection[*DeploymentResource]
	DeploymentResourceEdge       = pagination.Edge[*DeploymentResource]
)

type Deployment struct {
	CreatedAt       time.Time `json:"createdAt"`
	Repository      *string   `json:"repository"`
	UUID            uuid.UUID `json:"-"`
	TeamSlug        slug.Slug `json:"-"`
	EnvironmentName string    `json:"-"`
}

func (Deployment) IsNode() {}

func (d *Deployment) ID() ident.Ident {
	return newDeploymentIdent(d.UUID)
}

type DeploymentResource struct {
	CreatedAt time.Time `json:"createdAt"`
	Group     string    `json:"group"`
	Kind      string    `json:"kind"`
	Name      string    `json:"name"`
	Version   string    `json:"version"`
	Namespace string    `json:"namespace"`
}

type DeploymentStatus struct {
	Status  string    `json:"status"`
	Message *string   `json:"message,omitempty"`
	Created time.Time `json:"created"`
}

type DeploymentInfo struct {
	Deployer  *string    `json:"deployer,omitempty"`
	Timestamp *time.Time `json:"timestamp,omitempty"`
	CommitSha *string    `json:"commitSha,omitempty"`
	URL       *string    `json:"url,omitempty"`

	TeamSlug        slug.Slug     `json:"-"`
	EnvironmentName string        `json:"-"`
	WorkloadName    string        `json:"-"`
	WorkloadType    workload.Type `json:"-"`
}

type ChangeDeploymentKeyInput struct {
	TeamSlug slug.Slug `json:"team"`
}

type ChangeDeploymentKeyPayload struct {
	DeploymentKey *DeploymentKey `json:"deploymentKey,omitempty"`
}

type DeploymentKey struct {
	Key      string    `json:"key"`
	Created  time.Time `json:"created"`
	Expires  time.Time `json:"expires"`
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
	return newDeploymentKeyIdent(d.TeamSlug)
}

func (DeploymentKey) IsNode() {}

func toGraphDeployment(row *deploymentsql.Deployment) *Deployment {
	return &Deployment{
		CreatedAt:       row.CreatedAt.Time,
		Repository:      row.Repository,
		UUID:            row.ID,
		TeamSlug:        row.TeamSlug,
		EnvironmentName: row.Environment,
	}
	/*
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
			Statuses:        statuses,
			Resources:       resources,
			ExternalID:      d.DeploymentInfo.ID,
		}

	*/
}

func toGraphDeploymentResource(row *deploymentsql.DeploymentK8sResource) *DeploymentResource {
	return &DeploymentResource{
		Group:     row.Group,
		Version:   row.Version,
		Kind:      row.Kind,
		Name:      row.Name,
		Namespace: row.Namespace,
	}
	/*
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
			Statuses:        statuses,
			Resources:       resources,
			ExternalID:      d.DeploymentInfo.ID,
		}

	*/
}
