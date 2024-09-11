package job

import (
	"fmt"
	"io"
	"strconv"

	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v1/graphv1/ident"
	"github.com/nais/api/internal/v1/graphv1/modelv1"
	"github.com/nais/api/internal/v1/graphv1/pagination"
	"github.com/nais/api/internal/v1/workload"
	"github.com/nais/liberator/pkg/apis/nais.io/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

type (
	JobConnection = pagination.Connection[*Job]
	JobEdge       = pagination.Edge[*Job]
)

type Job struct {
	workload.Base
	Resources *JobResources `json:"resources"`
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

type JobResources struct {
	Limits   *workload.WorkloadResourceQuantity `json:"limits"`
	Requests *workload.WorkloadResourceQuantity `json:"requests"`
}

func (JobResources) IsWorkloadResources() {}

func toGraphJobResources(resources *nais_io_v1.ResourceRequirements) *JobResources {
	ret := &JobResources{
		Limits:   &workload.WorkloadResourceQuantity{},
		Requests: &workload.WorkloadResourceQuantity{},
	}

	if resources == nil {
		return ret
	}

	if resources.Limits != nil {
		if q, err := resource.ParseQuantity(resources.Limits.Cpu); err == nil {
			ret.Limits.CPU = q.AsApproximateFloat64()
		}

		if m, err := resource.ParseQuantity(resources.Limits.Memory); err == nil {
			ret.Limits.Memory = m.Value()
		}
	}

	if resources.Requests != nil {
		if q, err := resource.ParseQuantity(resources.Requests.Cpu); err == nil {
			ret.Requests.CPU = q.AsApproximateFloat64()
		}

		if m, err := resource.ParseQuantity(resources.Requests.Memory); err == nil {
			ret.Requests.Memory = m.Value()
		}
	}

	return ret
}

func toGraphJob(job *nais_io_v1.Naisjob, environmentName string) *Job {
	return &Job{
		Base: workload.Base{
			Name:            job.Name,
			EnvironmentName: environmentName,
			TeamSlug:        slug.Slug(job.Namespace),
		},
		Resources: toGraphJobResources(job.Spec.Resources),
	}
}
