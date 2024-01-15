package model

import (
	"github.com/nais/api/internal/slug"
)

type Secret struct {
	Env  Env               `json:"env"`
	Name string            `json:"name"`
	Data map[string]string `json:"data"`
}

type SecretGQLVars struct {
	Team slug.Slug
}
