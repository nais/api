package serviceaccount

import (
	"time"

	"github.com/google/uuid"
	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/serviceaccount/serviceaccountsql"
	"github.com/nais/api/internal/slug"
)

type (
	ServiceAccountWorkloadBindingConnection = pagination.Connection[*ServiceAccountWorkloadBinding]
	ServiceAccountWorkloadBindingEdge       = pagination.Edge[*ServiceAccountWorkloadBinding]
)

// ServiceAccountWorkloadBinding represents a binding between a Nais service account and a Nais workload, allowing
// the workload to authenticate as the service account using its Kubernetes ServiceAccount token.
type ServiceAccountWorkloadBinding struct {
	UUID                        uuid.UUID  `json:"-"`
	ServiceAccountID            uuid.UUID  `json:"-"`
	Environment                 string     `json:"environment"`
	TeamSlug                    slug.Slug  `json:"teamSlug"`
	WorkloadName                string     `json:"workloadName"`
	KubernetesServiceAccountUID *uuid.UUID `json:"kubernetesServiceAccountUID,omitempty"`
	CreatedAt                   time.Time  `json:"createdAt"`
	UpdatedAt                   time.Time  `json:"updatedAt"`
	LastUsedAt                  *time.Time `json:"lastUsedAt,omitempty"`
}

func (ServiceAccountWorkloadBinding) IsNode() {}

func (b *ServiceAccountWorkloadBinding) ID() ident.Ident {
	return newBindingIdent(b.UUID)
}

type AddWorkloadToServiceAccountInput struct {
	ServiceAccountID ident.Ident `json:"serviceAccountID"`
	Environment      string      `json:"environment"`
	TeamSlug         slug.Slug   `json:"teamSlug"`
	WorkloadName     string      `json:"workloadName"`
}

type AddWorkloadToServiceAccountPayload struct {
	ServiceAccount *ServiceAccount                `json:"serviceAccount,omitempty"`
	Binding        *ServiceAccountWorkloadBinding `json:"binding,omitempty"`
}

type RemoveWorkloadFromServiceAccountInput struct {
	BindingID ident.Ident `json:"bindingID"`
}

type RemoveWorkloadFromServiceAccountPayload struct {
	ServiceAccount *ServiceAccount `json:"serviceAccount,omitempty"`
	BindingDeleted *bool           `json:"bindingDeleted,omitempty"`
}

func toGraphServiceAccountWorkloadBinding(b *serviceaccountsql.ServiceAccountWorkloadBinding) *ServiceAccountWorkloadBinding {
	var lastUsed *time.Time
	if b.LastUsedAt.Valid {
		lastUsed = &b.LastUsedAt.Time
	}
	return &ServiceAccountWorkloadBinding{
		UUID:                        b.ID,
		ServiceAccountID:            b.ServiceAccountID,
		Environment:                 b.Environment,
		TeamSlug:                    b.TeamSlug,
		WorkloadName:                b.WorkloadName,
		KubernetesServiceAccountUID: b.KubernetesServiceAccountUid,
		CreatedAt:                   b.CreatedAt.Time,
		UpdatedAt:                   b.UpdatedAt.Time,
		LastUsedAt:                  lastUsed,
	}
}
