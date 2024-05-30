package model

import (
	"time"

	"github.com/nais/api/internal/graph/scalar"
	"github.com/nais/api/internal/slug"
)

func (Secret) IsSearchNode() {}

type Secret struct {
	ID             scalar.Ident      `json:"id"` // This is a graphql ID, cahcing, deduplication etc
	Name           string            `json:"name"`
	Data           map[string]string `json:"data"`
	LastModifiedAt *time.Time        `json:"lastModifiedAt,omitempty"`

	GQLVars SecretGQLVars `json:"-"` // Internal context for custom resolvers
}

type SecretGQLVars struct {
	Env            string
	Team           slug.Slug
	LastModifiedBy string
}
