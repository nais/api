package serviceaccount

import (
	"github.com/nais/api/internal/graph/scalar"
	"time"

	"github.com/google/uuid"
	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/serviceaccount/serviceaccountsql"
	"github.com/nais/api/internal/slug"
)

type (
	ServiceAccountConnection = pagination.Connection[*ServiceAccount]
	ServiceAccountEdge       = pagination.Edge[*ServiceAccount]
)

type ServiceAccount struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	CreatedAt   time.Time  `json:"created_at"`
	UUID        uuid.UUID  `json:"-"`
	TeamSlug    *slug.Slug `json:"-"`
}

func (ServiceAccount) IsNode()                   {}
func (s *ServiceAccount) GetID() uuid.UUID       { return s.UUID }
func (s *ServiceAccount) Identity() string       { return s.Name }
func (s *ServiceAccount) IsServiceAccount() bool { return true }
func (s *ServiceAccount) ID() ident.Ident {
	return NewIdent(s.UUID)
}

type CreateServiceAccountInput struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	TeamSlug    *slug.Slug `json:"teamSlug,omitempty"`
}

type CreateServiceAccountPayload struct {
	ServiceAccount *ServiceAccount `json:"serviceAccount"`
}

type DeleteServiceAccountPayload struct {
	ServiceAccountDeleted *bool `json:"serviceAccountDeleted,omitempty"`
}

type AddRoleToServiceAccountInput struct {
	ServiceAccountID ident.Ident `json:"serviceAccountID"`
	RoleName         string      `json:"roleName"`
}

type AddRoleToServiceAccountPayload struct {
	ServiceAccount *ServiceAccount `json:"serviceAccount,omitempty"`
}

type CreateServiceAccountTokenInput struct {
	ServiceAccountID ident.Ident  `json:"serviceAccountID"`
	Note             string       `json:"note"`
	ExpiresAt        *scalar.Date `json:"expiresAt,omitempty"`
}

type CreateServiceAccountTokenPayload struct {
	ServiceAccountToken *ServiceAccountToken `json:"serviceAccountToken,omitempty"`
	Token               *string              `json:"token,omitempty"`
}

type DeleteServiceAccountInput struct {
	ID ident.Ident `json:"id"`
}

type DeleteServiceAccountTokenInput struct {
	ServiceAccountTokenID ident.Ident `json:"serviceAccountTokenID"`
}

type DeleteServiceAccountTokenPayload struct {
	ServiceAccountTokenDeleted *bool `json:"serviceAccountTokenDeleted,omitempty"`
}

type RemoveRoleFromServiceAccountInput struct {
	ServiceAccountID ident.Ident `json:"serviceAccountID"`
	RoleName         string      `json:"roleName"`
}

type RemoveRoleFromServiceAccountPayload struct {
	ServiceAccount *ServiceAccount `json:"serviceAccount,omitempty"`
}

type ServiceAccountToken struct {
	ID        ident.Ident  `json:"id"`
	Note      string       `json:"note"`
	CreatedAt time.Time    `json:"createdAt"`
	UpdatedAt *time.Time   `json:"updatedAt,omitempty"`
	ExpiresAt *scalar.Date `json:"expiresAt,omitempty"`
}

type UpdateServiceAccountInput struct {
	ID          ident.Ident `json:"id"`
	Name        *string     `json:"name,omitempty"`
	Description *string     `json:"description,omitempty"`
}

type UpdateServiceAccountPayload struct {
	ServiceAccount *ServiceAccount `json:"serviceAccount,omitempty"`
}

type UpdateServiceAccountTokenExpiresAtInput struct {
	ExpiresAt    *scalar.Date `json:"expiresAt,omitempty"`
	RemoveExpiry *bool        `json:"removeExpiry,omitempty"`
}

type UpdateServiceAccountTokenInput struct {
	ServiceAccountTokenID ident.Ident                              `json:"serviceAccountTokenID"`
	Note                  *string                                  `json:"note,omitempty"`
	ExpiresAt             *UpdateServiceAccountTokenExpiresAtInput `json:"expiresAt,omitempty"`
}

type UpdateServiceAccountTokenPayload struct {
	ServiceAccountToken *ServiceAccountToken `json:"serviceAccountToken,omitempty"`
}

func toGraphServiceAccount(s *serviceaccountsql.ServiceAccount) *ServiceAccount {
	return &ServiceAccount{
		UUID:        s.ID,
		CreatedAt:   s.CreatedAt.Time,
		TeamSlug:    s.TeamSlug,
		Name:        s.Name,
		Description: s.Description,
	}
}
