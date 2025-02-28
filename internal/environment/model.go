package environment

import (
	"github.com/nais/api/internal/environment/environmentsql"
	"github.com/nais/api/internal/graph/ident"
)

type Environment struct {
	Name string `json:"name"`
	GCP  bool   `json:"-"`
}

func (Environment) IsNode() {}

func (e Environment) ID() ident.Ident {
	return newIdent(e.Name)
}

func toGraphEnvironment(e *environmentsql.Environment) *Environment {
	return &Environment{
		Name: e.Name,
	}
}
