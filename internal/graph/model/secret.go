package model

import (
	"github.com/nais/api/internal/graph/scalar"
	"github.com/nais/api/internal/slug"
)

type Secret struct {
	ID      scalar.Ident      `json:"id"`  // TODO: What is the purpose of this?
	Env     Env               `json:"env"` // TODO: Why is this here in some models, and in GQLVars in other models?
	Name    string            `json:"name"`
	Data    map[string]string `json:"data"`
	GQLVars SecretGQLVars     `json:"-"` // TODO: What is the purpose of this? Should Env be a part of this?
}

type SecretGQLVars struct {
	Team slug.Slug
}
