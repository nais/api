package job

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/workload"
	nais_io_v1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

type (
	JobConnection            = pagination.Connection[*Job]
	JobEdge                  = pagination.Edge[*Job]
	JobRunConnection         = pagination.Connection[*JobRun]
	JobRunEdge               = pagination.Edge[*JobRun]
	JobRunInstanceConnection = pagination.Connection[*JobRunInstance]
	JobRunInstanceEdge       = pagination.Edge[*JobRunInstance]
)

type JobRunInstance struct {
	Name            string    `json:"name"`
	EnvironmentName string    `json:"-"`
	TeamSlug        slug.Slug `json:"-"`
}

func (JobRunInstance) IsNode() {}

type Job struct {
	workload.Base
	Spec *nais_io_v1.NaisjobSpec `json:"-"`
}

func (Job) IsNode()       {}
func (Job) IsSearchNode() {}
func (Job) IsWorkload()   {}

// GetSecrets returns a list of secret names used by the job
func (j *Job) GetSecrets() []string {
	ret := make([]string, 0)
	for _, v := range j.Spec.EnvFrom {
		ret = append(ret, v.Secret)
	}
	for _, v := range j.Spec.FilesFrom {
		ret = append(ret, v.Secret)
	}
	return ret
}

type JobManifest struct {
	Content string `json:"content"`
}

func (JobManifest) IsWorkloadManifest() {}

type JobSchedule struct {
	Expression string `json:"expression"`
	TimeZone   string `json:"timeZone"`
}

type JobRun struct {
	Name            string         `json:"name"`
	StartTime       *time.Time     `json:"startTime"`
	Trigger         *JobRunTrigger `json:"-"`
	CreationTime    time.Time      `json:"-"`
	EnvironmentName string         `json:"-"`
	TeamSlug        slug.Slug      `json:"-"`
	Failed          bool           `json:"-"`
	Message         string         `json:"-"`
	Ready           int            `json:"ready"`

	spec *batchv1.Job
}

func (JobRun) IsNode() {}

func (j JobRun) ID() ident.Ident {
	return newJobRunIdent(j.TeamSlug, j.EnvironmentName, j.Name)
}

func (j JobRunInstance) ID() ident.Ident {
	return newJobRunInstanceIdent(j.TeamSlug, j.EnvironmentName, j.Name)
}

func (j *JobRun) Status() *JobRunStatus {
	if j.spec.Status.StartTime == nil {
		return &JobRunStatus{
			State:   JobRunStatePending,
			Message: "Pending",
		}
	}

	if ptr.Deref(j.spec.Status.Ready, 0) > 0 || ptr.Deref(j.spec.Status.Terminating, 0) > 0 {
		return &JobRunStatus{
			State:   JobRunStateRunning,
			Message: "Running",
		}
	}

	if j.spec.Status.CompletionTime == nil {
		for _, cs := range j.spec.Status.Conditions {
			if cs.Status == corev1.ConditionTrue && cs.Type == batchv1.JobFailed {
				return &JobRunStatus{
					State:   JobRunStateFailed,
					Message: cs.Message,
				}
			}
		}

		return &JobRunStatus{
			State:   JobRunStateRunning,
			Message: "Running",
		}
	}

	return &JobRunStatus{
		State:   JobRunStateSucceeded,
		Message: "Succeeded",
	}
}

func (j *JobRun) CompletionTime() *time.Time {
	switch j.Status().State {
	case JobRunStateSucceeded:
		return &j.spec.Status.CompletionTime.Time
	case JobRunStateFailed:
		for _, cs := range j.spec.Status.Conditions {
			if cs.Status == corev1.ConditionTrue && cs.Type == batchv1.JobFailed {
				return &cs.LastTransitionTime.Time
			}
		}

		return nil
	default:
		return nil
	}
}

func (j *JobRun) Image() *workload.ContainerImage {
	name, tag, _ := strings.Cut(j.spec.Spec.Template.Spec.Containers[0].Image, ":")
	return &workload.ContainerImage{
		Name: name,
		Tag:  tag,
	}
}

// Duration returns the duration of the job run.
func (j *JobRun) Duration() time.Duration {
	if j.spec.Status.StartTime == nil {
		return 0
	}

	if j.spec.Status.CompletionTime != nil {
		return j.spec.Status.CompletionTime.Sub(j.spec.Status.StartTime.Time)
	}

	if failed := failedTime(j.spec); failed != nil {
		return failed.Sub(j.spec.Status.StartTime.Time)
	}

	return time.Since(j.spec.Status.StartTime.Time)
}

type JobRunState int

const (
	JobRunStateUnknown JobRunState = iota
	JobRunStatePending
	JobRunStateRunning
	JobRunStateSucceeded
	JobRunStateFailed
)

func (e JobRunState) IsValid() bool {
	switch e {
	case JobRunStateUnknown, JobRunStatePending, JobRunStateRunning, JobRunStateSucceeded, JobRunStateFailed:
		return true
	}
	return false
}

func (e JobRunState) String() string {
	switch e {
	case JobRunStatePending:
		return "PENDING"
	case JobRunStateRunning:
		return "RUNNING"
	case JobRunStateSucceeded:
		return "SUCCEEDED"
	case JobRunStateFailed:
		return "FAILED"
	default:
		return "UNKNOWN"
	}
}

func (e *JobRunState) UnmarshalGQL(v interface{}) error {
	panic("not implemented")
}

func (e JobRunState) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

func (j Job) ID() ident.Ident {
	return newIdent(j.TeamSlug, j.EnvironmentName, j.Name)
}

type JobOrder struct {
	Field     JobOrderField        `json:"field"`
	Direction model.OrderDirection `json:"direction"`
}

type JobOrderField string

func (e JobOrderField) IsValid() bool {
	return SortFilter.SupportsSort(e)
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
			ret.Limits.CPU = ptr.To(q.AsApproximateFloat64())
		}

		if m, err := resource.ParseQuantity(resources.Limits.Memory); err == nil {
			ret.Limits.Memory = ptr.To(m.Value())
		}
	}

	if resources.Requests != nil {
		if q, err := resource.ParseQuantity(resources.Requests.Cpu); err == nil {
			ret.Requests.CPU = ptr.To(q.AsApproximateFloat64())
		}

		if m, err := resource.ParseQuantity(resources.Requests.Memory); err == nil {
			ret.Requests.Memory = ptr.To(m.Value())
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
	getConditions := func(status nais_io_v1.Status) []metav1.Condition {
		if status.Conditions == nil {
			return nil
		}

		return *status.Conditions
	}

	var logging *nais_io_v1.Logging
	if job.Spec.Observability != nil && job.Spec.Observability.Logging != nil {
		logging = job.Spec.Observability.Logging
	}

	var deletedAt *time.Time
	if job.DeletionTimestamp != nil {
		deletedAt = ptr.To(job.DeletionTimestamp.Time)
	}

	imageString := job.GetEffectiveImage()
	if len(imageString) == 0 {
		imageString = job.GetImage()
	}

	return &Job{
		Base: workload.Base{
			Name:                job.Name,
			DeletionStartedAt:   deletedAt,
			EnvironmentName:     environmentName,
			TeamSlug:            slug.Slug(job.Namespace),
			ImageString:         imageString,
			Conditions:          getConditions(job.Status),
			AccessPolicy:        job.Spec.AccessPolicy,
			Annotations:         job.GetAnnotations(),
			RolloutCompleteTime: job.GetStatus().RolloutCompleteTime,
			Type:                workload.TypeJob,
			Logging:             logging,
		},
		Spec: &job.Spec,
	}
}

func toGraphJobRun(run *batchv1.Job, environmentName string) *JobRun {
	var startTime *time.Time

	if run.Status.StartTime != nil {
		startTime = &run.Status.StartTime.Time
	}

	return &JobRun{
		Name:            run.Name,
		EnvironmentName: environmentName,
		TeamSlug:        slug.Slug(run.Namespace),
		StartTime:       startTime,
		CreationTime:    run.CreationTimestamp.Time,
		Trigger:         trigger(run),
		Failed:          run.Status.Failed > 0,
		Message:         statusMessage(run),
		spec:            run,
	}
}

func trigger(run *batchv1.Job) *JobRunTrigger {
	triggerType := JobRunTriggerTypeAutomatic
	actor := ""

	annotations := run.GetAnnotations()
	for k, v := range annotations {
		if k == "cronjob.kubernetes.io/instantiate" && v == "manual" {
			triggerType = JobRunTriggerTypeManual
			actor = annotations["nais.io/instantiated-by"]
		}
	}
	return &JobRunTrigger{
		Type:  triggerType,
		Actor: &actor,
	}
}

func toGraphJobRunInstance(run *corev1.Pod, environmentName string) *JobRunInstance {
	return &JobRunInstance{
		Name:            run.Name,
		EnvironmentName: environmentName,
		TeamSlug:        slug.Slug(run.Namespace),
	}
}

type DeleteJobInput struct {
	Name            string    `json:"name"`
	TeamSlug        slug.Slug `json:"teamSlug"`
	EnvironmentName string    `json:"environmentName"`
}

type DeleteJobPayload struct {
	TeamSlug *slug.Slug `json:"-"`
	Success  bool       `json:"success"`
}

type TriggerJobInput struct {
	Name            string    `json:"name"`
	TeamSlug        slug.Slug `json:"teamSlug"`
	EnvironmentName string    `json:"environmentName"`
	RunName         string    `json:"runName"`
}

type TriggerJobPayload struct {
	JobName         string    `json:"jobName"`
	TeamSlug        slug.Slug `json:"teamSlug"`
	EnvironmentName string    `json:"environmentName"`
	JobRun          *JobRun   `json:"jobRun"`
}

func statusMessage(job *batchv1.Job) string {
	if failedTime(job) != nil {
		return fmt.Sprintf("Run failed after %d attempts", job.Status.Failed)
	}
	target := completionTarget(job)
	if job.Status.Active > 0 {
		msg := ""
		if job.Status.Active == 1 {
			msg = "1 instance running"
		} else {
			msg = fmt.Sprintf("%d instances running", job.Status.Active)
		}
		return fmt.Sprintf("%s. %d/%d completed (%d failed %s)", msg, job.Status.Succeeded, target, job.Status.Failed, pluralize("attempt", job.Status.Failed))
	} else if job.Status.Succeeded == target {
		return fmt.Sprintf("%d/%d instances completed (%d failed %s)", job.Status.Succeeded, target, job.Status.Failed, pluralize("attempt", job.Status.Failed))
	}
	return ""
}

// failedTime returns a possible timestamp representing when the job run failed.
func failedTime(job *batchv1.Job) *time.Time {
	for _, cond := range job.Status.Conditions {
		if cond.Status == corev1.ConditionTrue && cond.Type == batchv1.JobFailed {
			return &cond.LastTransitionTime.Time
		}
	}
	return nil
}

// completionTarget is the number of successful runs we want to see based on parallelism and completions
func completionTarget(job *batchv1.Job) int32 {
	if job.Spec.Completions == nil && job.Spec.Parallelism == nil {
		return 1
	}
	if job.Spec.Completions != nil {
		return *job.Spec.Completions
	}
	return *job.Spec.Parallelism
}

func pluralize(s string, count int32) string {
	if count == 1 {
		return s
	}
	return s + "s"
}

type TeamInventoryCountJobs struct {
	Total    int       `json:"total"`
	TeamSlug slug.Slug `json:"-"`
}

type JobRunStatus struct {
	State   JobRunState `json:"state"`
	Message string      `json:"message"`
}

type TeamJobsFilter struct {
	Name         string   `json:"name"`
	Environments []string `json:"environments"`
}

type JobRunTrigger struct {
	// The type of trigger that started the job.
	Type JobRunTriggerType `json:"type"`
	// The actor/user who triggered the job run manually, if applicable.
	Actor *string `json:"actor,omitempty"`
}

type JobRunTriggerType string

const (
	JobRunTriggerTypeAutomatic JobRunTriggerType = "AUTOMATIC"
	JobRunTriggerTypeManual    JobRunTriggerType = "MANUAL"
)

var AllJobRunTriggerType = []JobRunTriggerType{
	JobRunTriggerTypeAutomatic,
	JobRunTriggerTypeManual,
}

func (e JobRunTriggerType) IsValid() bool {
	switch e {
	case JobRunTriggerTypeAutomatic, JobRunTriggerTypeManual:
		return true
	}
	return false
}

func (e JobRunTriggerType) String() string {
	return string(e)
}

func (e *JobRunTriggerType) UnmarshalGQL(v any) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = JobRunTriggerType(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid JobRunTriggerType", str)
	}
	return nil
}

func (e JobRunTriggerType) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}
