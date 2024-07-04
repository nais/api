package application

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strconv"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/slug"

	"github.com/nais/api/internal/graphv1/modelv1"
	"github.com/nais/api/internal/graphv1/pagination"
)

type (
	ApplicationConnection = pagination.Connection[*Application]
	ApplicationEdge       = pagination.Edge[*Application]
)

type Workload interface {
	IsWorkload()
}

type Application struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	EnvironmentName string    `json:"-"`
	TeamSlug        slug.Slug `json:"-"`
}

func (Application) IsWorkload() {}

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

var AllApplicationOrderField = []ApplicationOrderField{
	ApplicationOrderFieldStatus,
	ApplicationOrderFieldName,
	ApplicationOrderFieldEnvironment,
	ApplicationOrderFieldVulnerabilities,
	ApplicationOrderFieldRiskScore,
	ApplicationOrderFieldDeploymentTime,
}

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
	buf := &bytes.Buffer{}
	a.ID.MarshalGQLContext(context.TODO(), buf)
	id, _ := strconv.Unquote(buf.String())
	return &Application{
		ID:              id,
		Name:            a.Name,
		EnvironmentName: a.Env.Name,
		TeamSlug:        a.GQLVars.Team,
	}
}
