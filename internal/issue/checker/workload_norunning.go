package checker

import (
	"github.com/nais/api/internal/issue"
	nais_io_v1alpha1 "github.com/nais/liberator/pkg/apis/nais.io/v1alpha1"
	v1 "k8s.io/api/core/v1"
)

// noRunningInstances checks whether an application has no running instances.
// pods must already be fetched by the caller (e.g. Run).
func (w Workload) noRunningInstances(app *nais_io_v1alpha1.Application, pods []*v1.Pod, team, env string) *Issue {
	failing := failingPods(pods, app.Name)

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
