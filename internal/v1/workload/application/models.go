package application

import (
	"fmt"
	"io"
	"strconv"

	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v1/graphv1/ident"
	"github.com/nais/api/internal/v1/graphv1/modelv1"
	"github.com/nais/api/internal/v1/graphv1/pagination"
	"github.com/nais/api/internal/v1/workload"
	nais_io_v1alpha1 "github.com/nais/liberator/pkg/apis/nais.io/v1alpha1"
)

type (
	ApplicationConnection = pagination.Connection[*Application]
	ApplicationEdge       = pagination.Edge[*Application]
)

type Application struct {
	workload.Base
}

func (Application) IsWorkload() {}
func (Application) IsNode()     {}

func (a Application) ID() ident.Ident {
	return newIdent(a.TeamSlug, a.EnvironmentName, a.Name)
}

type ApplicationOrder struct {
	Field     ApplicationOrderField  `json:"field"`
	Direction modelv1.OrderDirection `json:"direction"`
}

type ApplicationOrderField string

const (
	ApplicationOrderFieldName           ApplicationOrderField = "NAME"
	ApplicationOrderFieldStatus         ApplicationOrderField = "STATUS"
	ApplicationOrderFieldEnvironment    ApplicationOrderField = "ENVIRONMENT"
	ApplicationOrderFieldDeploymentTime ApplicationOrderField = "DEPLOYMENT_TIME"
)

func (e ApplicationOrderField) IsValid() bool {
	switch e {
	case ApplicationOrderFieldStatus, ApplicationOrderFieldName, ApplicationOrderFieldEnvironment, ApplicationOrderFieldDeploymentTime:
		return true
	}
	return false
}

func (e ApplicationOrderField) String() string {
	return string(e)
}

func (e *ApplicationOrderField) UnmarshalGQL(v interface{}) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = ApplicationOrderField(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid ApplicationOrderField", str)
	}
	return nil
}

func (e ApplicationOrderField) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

func toGraphApplication(a *nais_io_v1alpha1.Application, environmentName string) *Application {
	return &Application{
		Base: workload.Base{
			Name:            a.Name,
			EnvironmentName: environmentName,
			TeamSlug:        slug.Slug(a.Namespace),
			ImageString:     a.Spec.Image,
		},
	}
}
