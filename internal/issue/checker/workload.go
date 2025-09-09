package checker

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/nais/api/internal/environmentmapper"
	"github.com/nais/api/internal/issue"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/workload/job"
	nais_io_v1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
	nais_io_v1alpha1 "github.com/nais/liberator/pkg/apis/nais.io/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

var deprecatedIngressList = map[string][]string{
	"dev-fss": {
		"adeo.no",
		"intern.dev.adeo.no",
		"dev-fss.nais.io",
		"dev.adeo.no",
		"dev.intern.nav.no",
		"nais.preprod.local",
	},
	"dev-gcp": {
		"dev-gcp.nais.io",
		"dev.intern.nav.no",
		"dev.nav.no",
		"intern.nav.no",
		"dev.adeo.no",
		"labs.nais.io",
		"ekstern.dev.nais.io",
	},
	"prod-fss": {
		"adeo.no",
		"nais.adeo.no",
		"prod-fss.nais.io",
	},
	"prod-gcp": {
		"dev.intern.nav.no",
		"prod-gcp.nais.io",
	},
}

var allowedRegistries = []string{
	"europe-north1-docker.pkg.dev",
	"repo.adeo.no:5443",
	"oliver006/redis_exporter",
	"bitnami/redis",
	"docker.io/oliver006/redis_exporter",
	"docker.io/redis",
	"docker.io/bitnami/redis",
	"redis",
}

type Workload struct {
	ApplicationLister KubernetesLister[*watcher.EnvironmentWrapper[*nais_io_v1alpha1.Application]]
	JobLister         KubernetesLister[*watcher.EnvironmentWrapper[*nais_io_v1.Naisjob]]
	PodWatcher        watcher.Watcher[*v1.Pod]
	RunWatcher        watcher.Watcher[*batchv1.Job]
}

var _ check = Workload{}

func (w Workload) Run(ctx context.Context) ([]Issue, error) {
	var ret []Issue
	for _, app := range w.ApplicationLister.List(ctx) {
		env := environmentmapper.EnvironmentName(app.Cluster)
		ret = deprecatedIngress(app.Obj, env, ret)
		ret = deprecatedRegistry(app.Obj.Spec.Image, app.Obj.Name, app.Obj.Namespace, env, issue.ResourceTypeApplication, ret)
		ret = w.noRunningInstances(app.Obj, app.Obj.Namespace, env, ret)
	}

	for _, job := range w.JobLister.List(ctx) {
		env := environmentmapper.EnvironmentName(job.Cluster)
		ret = deprecatedRegistry(job.Obj.Spec.Image, job.Obj.Name, job.Obj.Namespace, env, issue.ResourceTypeJob, ret)
		ret = w.failedJobRuns(job.GetName(), job.GetNamespace(), env, ret)
	}

	return ret, nil
}

func (w Workload) failedJobRuns(name, team, env string, ret []Issue) []Issue {
	// TODO: Should we limit how many runs we check?
	nameReq, err := labels.NewRequirement("app", selection.Equals, []string{name})
	if err != nil {
		return ret
	}

	selector := labels.NewSelector().Add(*nameReq)
	runs := w.RunWatcher.GetByNamespace(team, watcher.InCluster(env), watcher.WithLabels(selector))

	var tmpTime time.Time
	var tmpRun *job.JobRun

	for _, run := range runs {
		j := job.ToGraphJobRun(run.Obj, env)
		if j.StartTime != nil && j.StartTime.After(tmpTime) {
			tmpTime = *j.StartTime
			tmpRun = j
		} else {
			continue
		}
	}

	if tmpRun == nil {
		// No runs found, workload is not failing
		return ret
	}

	if tmpRun.Status().State == job.JobRunStateRunning {
		// Job is actively running
		return ret
	}

	if tmpRun.Failed {
		// Job run has failed
		return append(ret, Issue{
			IssueType:    issue.IssueTypeFailedJobRuns,
			ResourceName: name,
			ResourceType: issue.ResourceTypeJob,
			Team:         team,
			Env:          env,
			Severity:     issue.SeverityWarning,
			Message:      fmt.Sprintf("Job has failing runs. Last run '%s' failed with message: %s", tmpRun.Name, tmpRun.Message),
		})
	}
	return ret
}

func (w Workload) noRunningInstances(app *nais_io_v1alpha1.Application, team, env string, ret []Issue) []Issue {
	nameReq, err := labels.NewRequirement("app", selection.Equals, []string{app.Name})
	if err != nil {
		return ret
	}

	pods := w.PodWatcher.GetByNamespace(
		team,
		watcher.WithLabels(labels.NewSelector().Add(*nameReq)),
		watcher.InCluster(env),
	)

	failing := failingPods(watcher.Objects(pods), app.Name)

	hasReplicas := app.Spec.Replicas != nil &&
		app.Spec.Replicas.Min != nil && *app.Spec.Replicas.Min > 0 &&
		app.Spec.Replicas.Max != nil && *app.Spec.Replicas.Max > 0

	hasNoRunning := (len(pods) == 0 || len(failing) == len(pods)) && hasReplicas

	if hasNoRunning {
		return append(ret, Issue{
			IssueType:    issue.IssueTypeNoRunningInstances,
			ResourceName: app.Name,
			ResourceType: issue.ResourceTypeApplication,
			Team:         team,
			Env:          env,
			Severity:     issue.SeverityCritical,
			Message:      "Application has no running instances",
		})
	}

	return ret
}

func failingPods(pods []*v1.Pod, appName string) []*v1.Pod {
	var ret []*v1.Pod
	for _, pod := range pods {
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.Name == appName && cs.State.Running == nil {
				ret = append(ret, pod)
				break
			}
		}
	}
	return ret
}

func deprecatedRegistry(image, name, team, env string, resourceType issue.ResourceType, ret []Issue) []Issue {
	for _, registry := range allowedRegistries {
		if strings.HasPrefix(image, registry) {
			return ret
		}
	}

	return append(ret, Issue{
		IssueType:    issue.IssueTypeDeprecatedRegistry,
		ResourceName: name,
		ResourceType: resourceType,
		Team:         team,
		Env:          env,
		Severity:     issue.SeverityWarning,
		Message:      fmt.Sprintf("Image '%s' is using a deprecated registry", image),
	})
}

func deprecatedIngress(app *nais_io_v1alpha1.Application, env string, ret []Issue) []Issue {
	deprecated := func(ingresses []nais_io_v1.Ingress, env string) []string {
		ret := make([]string, 0)
		for _, ingress := range ingresses {
			i := strings.Join(strings.Split(string(ingress), ".")[1:], ".")
			for _, deprecatedIngress := range deprecatedIngressList[env] {
				if strings.HasPrefix(i, deprecatedIngress) {
					ret = append(ret, string(ingress))
				}
			}
		}
		return ret
	}

	di := deprecated(app.Spec.Ingresses, env)

	if len(di) == 0 {
		return ret
	}

	return append(ret, Issue{
		IssueType:    issue.IssueTypeDeprecatedIngress,
		ResourceName: app.Name,
		ResourceType: issue.ResourceTypeApplication,
		Team:         app.GetNamespace(),
		Env:          env,
		Severity:     issue.SeverityTodo,
		Message:      fmt.Sprintf("Application is using deprecated ingresses: %v", di),
		IssueDetails: issue.DeprecatedIngressIssueDetails{
			Ingresses: di,
		},
	})
}
