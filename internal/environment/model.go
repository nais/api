package environment

import (
	"fmt"
	"io"
	"strconv"

	"github.com/nais/api/internal/environment/environmentsql"
	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/pagination"
)

type (
	EnvironmentConnection = pagination.Connection[*Environment]
	EnvironmentEdge       = pagination.Edge[*Environment]
)

type Environment struct {
	Name string `json:"name"`
	GCP  bool   `json:"-"`
}

func (Environment) IsNode() {}

func (e Environment) ID() ident.Ident {
	return newIdent(e.Name)
}

type EnvironmentOrder struct {
	Field     EnvironmentOrderField `json:"field"`
	Direction model.OrderDirection  `json:"direction"`
}

type EnvironmentOrderField string

func (e EnvironmentOrderField) IsValid() bool {
	return SortFilter.SupportsSort(e)
}

func (e EnvironmentOrderField) String() string {
	return string(e)
}

func (e *EnvironmentOrderField) UnmarshalGQL(v any) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = EnvironmentOrderField(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid EnvironmentOrderField", str)
	}
	return nil
}

func (e EnvironmentOrderField) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

func toGraphEnvironment(e *environmentsql.Environment) *Environment {
	return &Environment{
		Name: e.Name,
	}
}
