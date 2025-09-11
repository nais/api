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
	libevents "github.com/nais/liberator/pkg/events"
	"github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

type Workload struct {
	ApplicationLister KubernetesLister[*watcher.EnvironmentWrapper[*nais_io_v1alpha1.Application]]
	JobLister         KubernetesLister[*watcher.EnvironmentWrapper[*nais_io_v1.Naisjob]]
	PodWatcher        watcher.Watcher[*v1.Pod]
	RunWatcher        watcher.Watcher[*batchv1.Job]
	log               logrus.FieldLogger
}

func (w Workload) Run(ctx context.Context) ([]Issue, error) {
	var ret []Issue
	for _, app := range w.ApplicationLister.List(ctx) {
		env := environmentmapper.EnvironmentName(app.Cluster)
		ret = appendIssues(ret, deprecatedIngress(app.Obj, env))
		ret = appendIssues(ret, deprecatedRegistry(app.Obj.Spec.Image, app.Obj.Name, app.Obj.Namespace, env, issue.ResourceTypeApplication))
		ret = appendIssues(ret, w.noRunningInstances(app.Obj, app.Obj.Namespace, env))
		ret = w.specErrors(app.Obj, env, ret)
	}

	for _, job := range w.JobLister.List(ctx) {
		env := environmentmapper.EnvironmentName(job.Cluster)
		ret = appendIssues(ret, deprecatedRegistry(job.Obj.Spec.Image, job.Obj.Name, job.Obj.Namespace, env, issue.ResourceTypeJob))
		ret = appendIssues(ret, w.failedJobRuns(job.GetName(), job.GetNamespace(), env))
	}

	return ret, nil
}

func (w Workload) lastRun(jobName, team, env string) (*job.JobRun, error) {
	nameReq, err := labels.NewRequirement("app", selection.Equals, []string{jobName})
	if err != nil {
		return nil, fmt.Errorf("create label requirement: %w", err)
	}

	selector := labels.NewSelector().Add(*nameReq)
	runs := w.RunWatcher.GetByNamespace(team, watcher.InCluster(env), watcher.WithLabels(selector))

	var latestTime time.Time
	var latest *job.JobRun

	for _, run := range runs {
		j := job.ToGraphJobRun(run.Obj, env)
		if j.StartTime != nil && j.StartTime.After(latestTime) {
			latestTime = *j.StartTime
			latest = j
		}
	}
	return latest, nil
}
func (w Workload) specErrors(app *nais_io_v1alpha1.Application, env string, ret []Issue) []Issue {
	if app == nil {
		return ret
	}
	if app.GetStatus() == nil {
		return ret
	}
	if app.GetStatus().Conditions == nil {
		return ret
	}
	condition, ok := w.condition(*app.GetStatus().Conditions)
	if !ok {
		return ret
	}

	switch condition.Reason {
	// A FailedGenerate error is almost always because of invalid yaml.
	case libevents.FailedGenerate:
		return append(ret, Issue{
			IssueType:    issue.IssueTypeInvalidSpec,
			ResourceName: app.Name,
			ResourceType: issue.ResourceTypeApplication,
			Team:         app.Namespace,
			Env:          env,
			Severity:     issue.SeverityCritical,
			Message:      condition.Message,
		})

	case libevents.FailedSynchronization:
		return append(ret, Issue{
			IssueType:    issue.IssueTypeFailedSynchronization,
			ResourceName: app.Name,
			ResourceType: issue.ResourceTypeApplication,
			Team:         app.Namespace,
			Env:          env,
			Severity:     issue.SeverityWarning,
			Message:      condition.Message,
		})
	}

	return ret
}

func (w Workload) condition(conditions []metav1.Condition) (metav1.Condition, bool) {
	for _, condition := range conditions {
		if condition.Type == "SynchronizationState" {
			return condition, true
		}
	}
	return metav1.Condition{}, false
}

func (w Workload) failedJobRuns(name, team, env string) *Issue {
	lastRun, err := w.lastRun(name, team, env)
	if err != nil {
		w.log.WithError(err).Error("fetch last job run")
		return nil
	}

	if lastRun == nil {
		return nil
	}

	if lastRun.Status().State == job.JobRunStateRunning {
		return nil
	}

	if lastRun.Failed {
		return &Issue{
			IssueType:    issue.IssueTypeFailedJobRuns,
			ResourceName: name,
			ResourceType: issue.ResourceTypeJob,
			Team:         team,
			Env:          env,
			Severity:     issue.SeverityWarning,
			Message:      fmt.Sprintf("Job has failing runs. Last run '%s' failed with message: %s", lastRun.Name, lastRun.Message),
		}
	}

	return nil
}

func (w Workload) noRunningInstances(app *nais_io_v1alpha1.Application, team, env string) *Issue {
	nameReq, err := labels.NewRequirement("app", selection.Equals, []string{app.Name})
	if err != nil {
		w.log.WithError(err).Error("create label requirement")
		return nil
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
		return &Issue{
			IssueType:    issue.IssueTypeNoRunningInstances,
			ResourceName: app.Name,
			ResourceType: issue.ResourceTypeApplication,
			Team:         team,
			Env:          env,
			Severity:     issue.SeverityCritical,
			Message:      "Application has no running instances",
		}
	}

	return nil
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

func deprecatedRegistry(image, name, team, env string, resourceType issue.ResourceType) *Issue {
	allowedRegistries := []string{
		"europe-north1-docker.pkg.dev",
		"repo.adeo.no:5443",
		"oliver006/redis_exporter",
		"bitnami/redis",
		"docker.io/oliver006/redis_exporter",
		"docker.io/redis",
		"docker.io/bitnami/redis",
		"redis",
	}

	for _, registry := range allowedRegistries {
		if strings.HasPrefix(image, registry) {
			return nil
		}
	}

	return &Issue{
		IssueType:    issue.IssueTypeDeprecatedRegistry,
		ResourceName: name,
		ResourceType: resourceType,
		Team:         team,
		Env:          env,
		Severity:     issue.SeverityWarning,
		Message:      fmt.Sprintf("Image '%s' is using a deprecated registry", image),
	}
}

func deprecatedIngress(app *nais_io_v1alpha1.Application, env string) *Issue {
	deprecatedIngressList := map[string][]string{
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
		return nil
	}

	return &Issue{
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
	}
}

// appendIssues appends issues to a slice, handling nil slices.
func appendIssues(slice []Issue, issues ...*Issue) []Issue {
	for _, issue := range issues {
		if issue != nil {
			slice = append(slice, *issue)
		}
	}
	return slice
}
