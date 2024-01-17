package model

import (
	"github.com/nais/api/internal/graph/scalar"
	"github.com/nais/api/internal/slug"
)

type Secret struct {
	ID      scalar.Ident      `json:"id"` // TODO: What is the purpose of this?
	Name    string            `json:"name"`
	Data    map[string]string `json:"data"`
	GQLVars SecretGQLVars     `json:"-"` // TODO: What is the purpose of this? Should Env be a part of this?
}

type EnvSecret struct {
	Env     Env      `json:"env"`
	Secrets []Secret `json:"secrets"`
}

type SecretGQLVars struct {
	Env  string
	Team slug.Slug
}
