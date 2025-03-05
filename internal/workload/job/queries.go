package job

import (
	"context"
	"fmt"
	"slices"

	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/workload"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	"sigs.k8s.io/yaml"
)

func ListAllForTeam(ctx context.Context, teamSlug slug.Slug, orderBy *JobOrder, filter *TeamJobsFilter) []*Job {
	allJobs := fromContext(ctx).jobWatcher.GetByNamespace(teamSlug.String())
	ret := make([]*Job, len(allJobs))
	for i, obj := range allJobs {
		ret[i] = toGraphJob(obj.Obj, obj.Cluster)
	}

	if filter != nil {
		ret = SortFilter.Filter(ctx, ret, filter)
	}

	if orderBy == nil {
		orderBy = &JobOrder{
			Field:     "NAME",
			Direction: model.OrderDirectionAsc,
		}
	}

	SortFilter.Sort(ctx, ret, orderBy.Field, orderBy.Direction)

	return ret
}

func ListAllInEnvironment(ctx context.Context, environment string) []*Job {
	jobs := fromContext(ctx).jobWatcher.GetByCluster(environment)
	ret := make([]*Job, len(jobs))
	for i, obj := range jobs {
		ret[i] = toGraphJob(obj.Obj, obj.Cluster)
	}
	return ret
}

func ListAllForTeamInEnvironment(ctx context.Context, teamSlug slug.Slug, environmentName string) []*Job {
	allJobs := fromContext(ctx).jobWatcher.GetByNamespace(teamSlug.String(), watcher.InCluster(environmentName))
	ret := make([]*Job, len(allJobs))
	for i, obj := range allJobs {
		ret[i] = toGraphJob(obj.Obj, obj.Cluster)
	}

	return ret
}

func ListJobRunInstances(ctx context.Context, teamSlug slug.Slug, environmentName, jobRunName string, page *pagination.Pagination) (*JobRunInstanceConnection, error) {
	pods, err := workload.ListAllPodsForJob(ctx, environmentName, teamSlug, jobRunName)
	if err != nil {
		return nil, err
	}

	converted := make([]*JobRunInstance, len(pods))
	for i, pod := range pods {
		converted[i] = toGraphJobRunInstance(pod, environmentName)
	}
	paginated := pagination.Slice(converted, page)
	return pagination.NewConnection(paginated, page, len(converted)), nil
}

func Get(ctx context.Context, teamSlug slug.Slug, environment, name string) (*Job, error) {
	job, err := fromContext(ctx).jobWatcher.Get(environment, teamSlug.String(), name)
	if err != nil {
		return nil, err
	}
	return toGraphJob(job, environment), nil
}

func GetJobRun(ctx context.Context, teamSlug slug.Slug, environment, name string) (*JobRun, error) {
	run, err := fromContext(ctx).runWatcher.Get(environment, teamSlug.String(), name)
	if err != nil {
		return nil, err
	}
	return toGraphJobRun(run, environment), nil
}

func GetByIdent(ctx context.Context, id ident.Ident) (*Job, error) {
	teamSlug, env, name, err := parseIdent(id)
	if err != nil {
		return nil, err
	}
	return Get(ctx, teamSlug, env, name)
}

func GetByJobRunIdent(ctx context.Context, id ident.Ident) (*JobRun, error) {
	teamSlug, env, name, err := parseIdent(id)
	if err != nil {
		return nil, err
	}
	return GetJobRun(ctx, teamSlug, env, name)
}

func getJobRunInstanceByIdent(ctx context.Context, id ident.Ident) (*JobRunInstance, error) {
	teamSlug, env, name, err := parseIdent(id)
	if err != nil {
		return nil, err
	}

	pod, err := workload.GetPod(ctx, env, teamSlug, name)
	if err != nil {
		return nil, err
	}

	return toGraphJobRunInstance(pod, env), nil
}

func Runs(ctx context.Context, teamSlug slug.Slug, environment, jobName string, page *pagination.Pagination) (*JobRunConnection, error) {
	nameReq, err := labels.NewRequirement("app", selection.Equals, []string{jobName})
	if err != nil {
		return nil, err
	}

	selector := labels.NewSelector().Add(*nameReq)

	allRuns := fromContext(ctx).runWatcher.GetByNamespace(teamSlug.String(), watcher.InCluster(environment), watcher.WithLabels(selector))

	ret := make([]*JobRun, len(allRuns))
	for i, run := range allRuns {
		ret[i] = toGraphJobRun(run.Obj, run.Cluster)
	}

	slices.SortStableFunc(ret, func(a, b *JobRun) int {
		if a.StartTime == nil {
			return 1
		}
		if b.StartTime == nil {
			return -1
		}

		return b.StartTime.Compare(*a.StartTime)
	})

	runs := pagination.Slice(ret, page)
	return pagination.NewConnection(runs, page, len(ret)), nil
}

func Manifest(ctx context.Context, teamSlug slug.Slug, environmentName, name string) (*JobManifest, error) {
	job, err := fromContext(ctx).jobWatcher.Get(environmentName, teamSlug.String(), name)
	if err != nil {
		return nil, err
	}

	manifest := map[string]any{
		"spec":       job.Spec,
		"apiVersion": job.APIVersion,
		"kind":       job.Kind,
		"metadata": map[string]any{
			"labels":    job.GetLabels(),
			"name":      name,
			"namespace": teamSlug.String(),
		},
	}

	b, err := yaml.Marshal(manifest)
	if err != nil {
		return nil, err
	}

	return &JobManifest{
		Content: string(b),
	}, nil
}

func Delete(ctx context.Context, teamSlug slug.Slug, environmentName, name string) (*DeleteJobPayload, error) {
	if err := fromContext(ctx).jobWatcher.Delete(ctx, environmentName, teamSlug.String(), name); err != nil {
		return nil, err
	}

	if err := activitylog.Create(ctx, activitylog.CreateInput{
		Action:          activitylog.ActivityLogEntryActionDeleted,
		Actor:           authz.ActorFromContext(ctx).User,
		ResourceType:    activityLogEntryResourceTypeJob,
		ResourceName:    name,
		EnvironmentName: &environmentName,
		TeamSlug:        &teamSlug,
	}); err != nil {
		return nil, err
	}

	return &DeleteJobPayload{
		TeamSlug: &teamSlug,
		Success:  true,
	}, nil
}

func Trigger(ctx context.Context, teamSlug slug.Slug, environmentName, name, runName string) (*JobRun, error) {
	w := fromContext(ctx).jobWatcher
	job, err := w.Get(environmentName, teamSlug.String(), name)
	if err != nil {
		return nil, err
	}

	if job.Spec.Schedule == "" {
		return nil, apierror.Errorf("Only Jobs with a schedule is supported")
	}

	cjClient, err := w.ImpersonatedClient(ctx, environmentName, watcher.WithImpersonatedClientGVR(batchv1.SchemeGroupVersion.WithResource("cronjobs")))
	if err != nil {
		return nil, fmt.Errorf("creating cronjob client: %w", err)
	}

	cronJob, err := cjClient.Namespace(teamSlug.String()).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting cronjob: %w", err)
	}

	jobRun, err := createJobFromCronJob(authz.ActorFromContext(ctx).User, runName, cronJob)
	if err != nil {
		return nil, fmt.Errorf("creating job from cronjob: %w", err)
	}

	jobClient, err := w.ImpersonatedClient(ctx, environmentName, watcher.WithImpersonatedClientGVR(batchv1.SchemeGroupVersion.WithResource("jobs")))
	if err != nil {
		return nil, fmt.Errorf("creating job client: %w", err)
	}

	if _, err = jobClient.Namespace(teamSlug.String()).Create(ctx, jobRun, metav1.CreateOptions{}); err != nil {
		return nil, err
	}

	if err := activitylog.Create(ctx, activitylog.CreateInput{
		Action:          activityLogEntryActionTriggerJob,
		Actor:           authz.ActorFromContext(ctx).User,
		ResourceType:    activityLogEntryResourceTypeJob,
		ResourceName:    name,
		EnvironmentName: &environmentName,
		TeamSlug:        &teamSlug,
	}); err != nil {
		return nil, err
	}

	jobRunBatch := &batchv1.Job{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(jobRun.Object, jobRunBatch); err != nil {
		return nil, err
	}

	return toGraphJobRun(jobRunBatch, environmentName), nil
}

// createJobFromCronJob creates a Job from a CronJob.
//
// Copied from https://github.com/kubernetes/kubectl/blob/5f5894cd61c609d7b55aa0f9bc99967155c69a9f/pkg/cmd/create/create_job.go#L254
func createJobFromCronJob(actor authz.AuthenticatedUser, name string, cronJob *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	annotations := make(map[string]string)
	annotations["cronjob.kubernetes.io/instantiate"] = "manual"
	annotations["nais.io/instantiated-by"] = actor.Identity()

	mp, ok, err := unstructured.NestedStringMap(cronJob.Object, "spec", "jobTemplate", "annotations")
	if err != nil {
		return nil, err
	}

	if ok {
		for k, v := range mp {
			annotations[k] = v
		}
	}

	labels, _, err := unstructured.NestedStringMap(cronJob.Object, "spec", "jobTemplate", "metadata", "labels")
	if err != nil {
		return nil, err
	}

	spec, ok, err := unstructured.NestedMap(cronJob.Object, "spec", "jobTemplate", "spec")
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("jobTemplate.spec not found")
	}

	job := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "batch/v1",
			"kind":       "Job",
			"metadata": map[string]any{
				"name":        name,
				"annotations": annotations,
				"labels":      labels,
				"ownerReferences": []map[string]any{
					{
						"apiVersion":         "batch/v1",
						"kind":               "CronJob",
						"name":               cronJob.GetName(),
						"uid":                cronJob.GetUID(),
						"controller":         true,
						"blockOwnerDeletion": true,
					},
				},
			},
			"spec": spec,
		},
	}
	b, err := yaml.Marshal(job)
	if err != nil {
		return nil, err
	}

	ret := &unstructured.Unstructured{}
	if err := yaml.Unmarshal(b, &ret.Object); err != nil {
		return nil, err
	}

	return ret, nil
}
