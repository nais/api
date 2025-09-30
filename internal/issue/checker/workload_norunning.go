package checker

import (
	"github.com/nais/api/internal/issue"
	"github.com/nais/api/internal/kubernetes/watcher"
	nais_io_v1alpha1 "github.com/nais/liberator/pkg/apis/nais.io/v1alpha1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

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

	hasReplicas := app.Spec.Replicas == nil || (app.Spec.Replicas.Min != nil && *app.Spec.Replicas.Min > 0 &&
		app.Spec.Replicas.Max != nil && *app.Spec.Replicas.Max > 0)

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
