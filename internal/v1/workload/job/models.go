package job

import (
	"fmt"
	"io"
	"strconv"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/v1/graphv1/ident"
	"github.com/nais/api/internal/v1/graphv1/modelv1"
	"github.com/nais/api/internal/v1/graphv1/pagination"
	"github.com/nais/api/internal/v1/workload"
)

type (
	JobConnection = pagination.Connection[*Job]
	JobEdge       = pagination.Edge[*Job]
)

type Job struct {
	workload.Base
}

func (Job) IsNode()     {}
func (Job) IsWorkload() {}

func (j Job) ID() ident.Ident {
	return newIdent(j.TeamSlug, j.EnvironmentName, j.Name)
}

type JobOrder struct {
	Field     JobOrderField          `json:"field"`
	Direction modelv1.OrderDirection `json:"direction"`
}

type JobOrderField string

const (
	JobOrderFieldStatus          JobOrderField = "STATUS"
	JobOrderFieldName            JobOrderField = "NAME"
	JobOrderFieldEnvironment     JobOrderField = "ENVIRONMENT"
	JobOrderFieldVulnerabilities JobOrderField = "VULNERABILITIES"
	JobOrderFieldRiskScore       JobOrderField = "RISK_SCORE"
	JobOrderFieldDeploymentTime  JobOrderField = "DEPLOYMENT_TIME"
)

func (e JobOrderField) IsValid() bool {
	switch e {
	case JobOrderFieldStatus, JobOrderFieldName, JobOrderFieldEnvironment, JobOrderFieldVulnerabilities, JobOrderFieldRiskScore, JobOrderFieldDeploymentTime:
		return true
	}
	return false
}

func (e JobOrderField) String() string {
	return string(e)
}

func (e *JobOrderField) UnmarshalGQL(v interface{}) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = JobOrderField(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid JobOrderField", str)
	}
	return nil
}

func (e JobOrderField) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

func toGraphJob(a *model.NaisJob) *Job {
	return &Job{
		Base: workload.Base{
			Name:            a.Name,
			EnvironmentName: a.Env.Name,
			TeamSlug:        a.GQLVars.Team,
		},
	}
}
