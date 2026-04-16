package checker

import (
	"context"

	"github.com/nais/api/internal/environmentmapper"
	"github.com/nais/api/internal/issue"
	"github.com/nais/api/internal/kubernetes/watcher"
	nais_io_v1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
	nais_io_v1alpha1 "github.com/nais/liberator/pkg/apis/nais.io/v1alpha1"
	"github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

type NaisWorkload interface {
	GetEffectiveImage() string
	GetImage() string
	GetName() string
	GetNamespace() string
	GetStatus() *nais_io_v1.Status
}

type Workload struct {
	AppWatcher     watcher.Watcher[*nais_io_v1alpha1.Application]
	IngressWatcher watcher.Watcher[*netv1.Ingress]
	JobWatcher     watcher.Watcher[*nais_io_v1.Naisjob]
	PodWatcher     watcher.Watcher[*v1.Pod]
	RunWatcher     watcher.Watcher[*batchv1.Job]
	V13sClient     V13sClient

	log logrus.FieldLogger
}

func (w Workload) Run(ctx context.Context) ([]Issue, error) {
	var ret []Issue
	w.AppWatcher.All()
	for _, app := range w.AppWatcher.All() {
		image, ok := image(app.Obj)
		if !ok {
			w.log.WithField("application", app.Obj.GetName()).WithField("namespace", app.Obj.GetNamespace()).Warn("application has no image")
			continue
		}
		env := environmentmapper.EnvironmentName(app.Cluster)

		pods := w.appPods(app.Obj, app.Obj.GetNamespace(), env)

		ret = appendIssues(ret, deprecatedIngress(app.Obj, env))
		ret = appendIssues(ret, deprecatedRegistry(image, app.Obj.GetName(), app.Obj.GetNamespace(), env, issue.ResourceTypeApplication))
		ret = appendIssues(ret, w.noRunningInstances(app.Obj, watcher.Objects(pods), app.Obj.GetNamespace(), env))
		ret = appendIssues(ret, w.restartLoop(app.Obj, watcher.Objects(pods), app.Obj.GetNamespace(), env))
		ret = appendIssues(ret, w.specErrors(app.Obj, env, issue.ResourceTypeApplication))
	}

	for _, job := range w.JobWatcher.All() {
		image, ok := image(job.Obj)
		if !ok {
			w.log.WithField("job", job.Obj.GetName()).WithField("namespace", job.Obj.GetNamespace()).Warn("job has no image")
			continue
		}
		env := environmentmapper.EnvironmentName(job.Cluster)
		ret = appendIssues(ret, deprecatedRegistry(image, job.Obj.Name, job.Obj.Namespace, env, issue.ResourceTypeJob))
		ret = appendIssues(ret, w.lastRunFailed(job.GetName(), job.GetNamespace(), env))
		ret = appendIssues(ret, w.specErrors(job.Obj, env, issue.ResourceTypeJob))
	}

	ret = appendIssues(ret, w.vulnerabilities(ctx)...)

	return ret, nil
}

// appPods fetches pods for the given application in the given namespace and environment.
func (w Workload) appPods(app *nais_io_v1alpha1.Application, team, env string) []*watcher.EnvironmentWrapper[*v1.Pod] {
	nameReq, err := labels.NewRequirement("app", selection.Equals, []string{app.Name})
	if err != nil {
		w.log.WithError(err).Error("create label requirement")
		return nil
	}
	return w.PodWatcher.GetByNamespace(
		team,
		watcher.WithLabels(labels.NewSelector().Add(*nameReq)),
		watcher.InCluster(env),
	)
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

func image(workload NaisWorkload) (string, bool) {
	if workload.GetImage() != "" {
		return workload.GetImage(), true
	}
	if workload.GetEffectiveImage() != "" {
		return workload.GetEffectiveImage(), true
	}
	return "", false
}
