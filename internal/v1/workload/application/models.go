package application

import (
	"fmt"
	"io"
	"strconv"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v1/graphv1/ident"
	"github.com/nais/api/internal/v1/graphv1/modelv1"
	"github.com/nais/api/internal/v1/graphv1/pagination"
)

type (
	ApplicationConnection = pagination.Connection[*Application]
	ApplicationEdge       = pagination.Edge[*Application]
)

type Application struct {
	Name            string    `json:"name"`
	EnvironmentName string    `json:"-"`
	TeamSlug        slug.Slug `json:"-"`
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
	ApplicationOrderFieldStatus          ApplicationOrderField = "STATUS"
	ApplicationOrderFieldName            ApplicationOrderField = "NAME"
	ApplicationOrderFieldEnvironment     ApplicationOrderField = "ENVIRONMENT"
	ApplicationOrderFieldVulnerabilities ApplicationOrderField = "VULNERABILITIES"
	ApplicationOrderFieldRiskScore       ApplicationOrderField = "RISK_SCORE"
	ApplicationOrderFieldDeploymentTime  ApplicationOrderField = "DEPLOYMENT_TIME"
)

func (e ApplicationOrderField) IsValid() bool {
	switch e {
	case ApplicationOrderFieldStatus, ApplicationOrderFieldName, ApplicationOrderFieldEnvironment, ApplicationOrderFieldVulnerabilities, ApplicationOrderFieldRiskScore, ApplicationOrderFieldDeploymentTime:
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

func toGraphApplication(a *model.App) *Application {
	return &Application{
		Name:            a.Name,
		EnvironmentName: a.Env.Name,
		TeamSlug:        a.GQLVars.Team,
	}
}
