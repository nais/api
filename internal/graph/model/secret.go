package model

import (
	"github.com/nais/api/internal/graph/scalar"
	"github.com/nais/api/internal/slug"
)

type Secret struct {
	ID   scalar.Ident      `json:"id"` // This is a graphql ID, cahcing, deduplication etc
	Name string            `json:"name"`
	Data map[string]string `json:"data"`
	Apps []*App            `json:"apps"`

	GQLVars SecretGQLVars `json:"-"` // Internal context for custom resolvers
}

type EnvSecret struct {
	Env     Env       `json:"env"`
	Secrets []*Secret `json:"secrets"`
}

type SecretGQLVars struct {
	Env  string
	Team slug.Slug
}
