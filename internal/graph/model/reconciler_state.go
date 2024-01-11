package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/nais/api/internal/slug"
)

// Reconciler state type.
type ReconcilerState struct {
	// The GitHub team slug.
	GitHubTeamSlug *slug.Slug `json:"gitHubTeamSlug,omitempty"`
	// The Google Workspace group email.
	GoogleWorkspaceGroupEmail *string `json:"googleWorkspaceGroupEmail,omitempty"`
	// The Azure AD group ID.
	AzureADGroupID *uuid.UUID `json:"azureADGroupId,omitempty"`
	// A list of GCP projects.
	GcpProjects []*GcpProject `json:"gcpProjects"`
	// A list of NAIS namespaces.
	NaisNamespaces []*NaisNamespace `json:"naisNamespaces"`
	// Timestamp of when the NAIS deploy key was provisioned.
	NaisDeployKeyProvisioned *time.Time `json:"naisDeployKeyProvisioned,omitempty"`
	// Name of the GAR repository for the team.
	GarRepositoryName *string `json:"garRepositoryName,omitempty"`
}
