package job

import (
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v1/graphv1/ident"
	"github.com/nais/api/internal/v1/graphv1/modelv1"
	"github.com/nais/api/internal/v1/graphv1/pagination"
	"github.com/nais/api/internal/v1/workload"
	"github.com/nais/liberator/pkg/apis/nais.io/v1"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"
)

type (
	JobConnection    = pagination.Connection[*Job]
	JobEdge          = pagination.Edge[*Job]
	JobRunConnection = pagination.Connection[*JobRun]
	JobRunEdge       = pagination.Edge[*JobRun]
)

type Job struct {
	workload.Base
	Spec *nais_io_v1.NaisjobSpec `json:"-"`
}

type JobSchedule struct {
	Expression string `json:"expression"`
	TimeZone   string `json:"timeZone"`
}

type JobRun struct {
	Name            string     `json:"name"`
	StartTime       *time.Time `json:"startTime"`
	CompletionTime  *time.Time `json:"completionTime"`
	CreationTime    time.Time  `json:"-"`
	EnvironmentName string     `json:"-"`
	TeamSlug        slug.Slug  `json:"-"`
}

func (JobRun) IsNode() {}

func (j JobRun) ID() ident.Ident {
	return newJobRunIdent(j.TeamSlug, j.EnvironmentName, j.Name)
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
	JobOrderFieldStatus         JobOrderField = "STATUS"
	JobOrderFieldName           JobOrderField = "NAME"
	JobOrderFieldEnvironment    JobOrderField = "ENVIRONMENT"
	JobOrderFieldDeploymentTime JobOrderField = "DEPLOYMENT_TIME"
)

func (e JobOrderField) IsValid() bool {
	switch e {
	case JobOrderFieldStatus, JobOrderFieldName, JobOrderFieldEnvironment, JobOrderFieldDeploymentTime:
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

func (j *Job) Resources() *JobResources {
	ret := &JobResources{
		Limits:   &workload.WorkloadResourceQuantity{},
		Requests: &workload.WorkloadResourceQuantity{},
	}

	resources := j.Spec.Resources
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

func (j *Job) Schedule() *JobSchedule {
	if j.Spec.Schedule == "" {
		return nil
	}

	return &JobSchedule{
		Expression: j.Spec.Schedule,
		TimeZone:   ptr.Deref(j.Spec.TimeZone, "UTC"),
	}
}

func toGraphJob(job *nais_io_v1.Naisjob, environmentName string) *Job {
	return &Job{
		Base: workload.Base{
			Name:            job.Name,
			EnvironmentName: environmentName,
			TeamSlug:        slug.Slug(job.Namespace),
		},
		Spec: &job.Spec,
	}
}

func toGraphJobRun(run *batchv1.Job, environmentName string) *JobRun {
	var startTime, completionTime *time.Time

	if run.Status.CompletionTime != nil {
		completionTime = &run.Status.CompletionTime.Time
	}

	if run.Status.StartTime != nil {
		startTime = &run.Status.StartTime.Time
	}

	/*
		podReq, err := labels.NewRequirement("job-name", selection.Equals, []string{job.Name})
		if err != nil {
			return nil, c.error(ctx, err, "creating label selector")
		}
		podSelector := labels.NewSelector().Add(*podReq)
		pods, err := c.informers[env].Pod.Lister().Pods(team).List(podSelector)
		if err != nil {
			return nil, c.error(ctx, err, "listing job instance pods")
		}

		var podNames []string
		for _, pod := range pods {
			podNames = append(podNames, pod.Name)
		}
	*/

	return &JobRun{
		Name:            run.Name,
		EnvironmentName: environmentName,
		TeamSlug:        slug.Slug(run.Namespace),
		StartTime:       startTime,
		CompletionTime:  completionTime,
		CreationTime:    run.CreationTimestamp.Time,
		/*
			PodNames:       podNames,
			Failed:         failed(job),
			Duration:       duration(job).String(),
			Image:          job.Spec.Template.Spec.Containers[0].Image,
			Message:        Message(job),
		*/
	}
}
