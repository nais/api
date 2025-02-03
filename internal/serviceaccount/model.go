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
	ServiceAccountConnection = pagination.Connection[*ServiceAccount]
	ServiceAccountEdge       = pagination.Edge[*ServiceAccount]
)

type ServiceAccount struct {
	UUID        uuid.UUID `json:"-"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	TeamSlug    slug.Slug `json:"-"`
}

func (ServiceAccount) IsNode()                   {}
func (s *ServiceAccount) GetID() uuid.UUID       { return s.UUID }
func (s *ServiceAccount) Identity() string       { return s.Name }
func (s *ServiceAccount) IsServiceAccount() bool { return true }
func (s *ServiceAccount) ID() ident.Ident {
	return NewIdent(s.UUID)
}

func toGraphServiceAccount(s *serviceaccountsql.ServiceAccount) *ServiceAccount {
	return &ServiceAccount{
		UUID: s.ID,
		Name: s.Name,
	}
}
