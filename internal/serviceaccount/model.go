package serviceaccount

import (
	"time"

	"github.com/google/uuid"
	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/graph/scalar"
	"github.com/nais/api/internal/serviceaccount/serviceaccountsql"
	"github.com/nais/api/internal/slug"
	"k8s.io/utils/ptr"
)

type (
	ServiceAccountConnection      = pagination.Connection[*ServiceAccount]
	ServiceAccountEdge            = pagination.Edge[*ServiceAccount]
	ServiceAccountTokenConnection = pagination.Connection[*ServiceAccountToken]
	ServiceAccountTokenEdge       = pagination.Edge[*ServiceAccountToken]
)

type ServiceAccount struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	UUID        uuid.UUID  `json:"-"`
	TeamSlug    *slug.Slug `json:"-"`
}

func (ServiceAccount) IsNode()                   {}
func (s *ServiceAccount) GetID() uuid.UUID       { return s.UUID }
func (s *ServiceAccount) Identity() string       { return s.Name }
func (s *ServiceAccount) IsServiceAccount() bool { return true }
func (s *ServiceAccount) IsAdmin() bool          { return false }
func (s *ServiceAccount) ID() ident.Ident {
	return newIdent(s.UUID)
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

type AssignRoleToServiceAccountInput struct {
	ServiceAccountID ident.Ident `json:"serviceAccountID"`
	RoleName         string      `json:"roleName"`
}

type AssignRoleToServiceAccountPayload struct {
	ServiceAccount *ServiceAccount `json:"serviceAccount,omitempty"`
}

type CreateServiceAccountTokenInput struct {
	ServiceAccountID ident.Ident  `json:"serviceAccountID"`
	Name             string       `json:"name"`
	Description      string       `json:"description"`
	ExpiresAt        *scalar.Date `json:"expiresAt,omitempty"`
}

type CreateServiceAccountTokenPayload struct {
	Secret              *string              `json:"secret,omitempty"`
	ServiceAccount      *ServiceAccount      `json:"serviceAccount,omitempty"`
	ServiceAccountToken *ServiceAccountToken `json:"serviceAccountToken,omitempty"`
}

type DeleteServiceAccountInput struct {
	ServiceAccountID ident.Ident `json:"serviceAccountID"`
}

type DeleteServiceAccountTokenInput struct {
	ServiceAccountTokenID ident.Ident `json:"serviceAccountTokenID"`
}

type DeleteServiceAccountTokenPayload struct {
	ServiceAccountTokenDeleted *bool           `json:"serviceAccountTokenDeleted,omitempty"`
	ServiceAccount             *ServiceAccount `json:"serviceAccount,omitempty"`
}

type RevokeRoleFromServiceAccountInput struct {
	ServiceAccountID ident.Ident `json:"serviceAccountID"`
	RoleName         string      `json:"roleName"`
}

type RevokeRoleFromServiceAccountPayload struct {
	ServiceAccount *ServiceAccount `json:"serviceAccount,omitempty"`
}

type ServiceAccountToken struct {
	Name             string       `json:"name"`
	Description      string       `json:"description"`
	CreatedAt        time.Time    `json:"createdAt"`
	UpdatedAt        time.Time    `json:"updatedAt,omitempty"`
	ExpiresAt        *scalar.Date `json:"expiresAt,omitempty"`
	UUID             uuid.UUID    `json:"-"`
	ServiceAccountID uuid.UUID    `json:"-"`
}

func (ServiceAccountToken) IsNode() {}
func (t *ServiceAccountToken) ID() ident.Ident {
	return newTokenIdent(t.UUID)
}

type UpdateServiceAccountInput struct {
	ServiceAccountID ident.Ident `json:"serviceAccountID"`
	Description      *string     `json:"description,omitempty"`
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
	Name                  *string                                  `json:"name,omitempty"`
	Description           *string                                  `json:"description,omitempty"`
	ExpiresAt             *UpdateServiceAccountTokenExpiresAtInput `json:"expiresAt,omitempty"`
}

type UpdateServiceAccountTokenPayload struct {
	ServiceAccount      *ServiceAccount      `json:"serviceAccount,omitempty"`
	ServiceAccountToken *ServiceAccountToken `json:"serviceAccountToken,omitempty"`
}

func toGraphServiceAccount(s *serviceaccountsql.ServiceAccount) *ServiceAccount {
	return &ServiceAccount{
		UUID:        s.ID,
		CreatedAt:   s.CreatedAt.Time,
		UpdatedAt:   s.UpdatedAt.Time,
		TeamSlug:    s.TeamSlug,
		Name:        s.Name,
		Description: s.Description,
	}
}

func toGraphServiceAccountToken(t *serviceaccountsql.ServiceAccountToken) *ServiceAccountToken {
	var expiresAt *scalar.Date
	if t.ExpiresAt.Valid {
		expiresAt = ptr.To(scalar.NewDate(t.ExpiresAt.Time))
	}

	return &ServiceAccountToken{
		Name:             t.Name,
		Description:      t.Description,
		CreatedAt:        t.CreatedAt.Time,
		UpdatedAt:        t.UpdatedAt.Time,
		ExpiresAt:        expiresAt,
		UUID:             t.ID,
		ServiceAccountID: t.ServiceAccountID,
	}
}
