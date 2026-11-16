package checker

import (
	"github.com/nais/api/internal/issue"
	"github.com/nais/api/internal/kubernetes/translate"
	nais_io_v1alpha1 "github.com/nais/liberator/pkg/apis/nais.io/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
)

// rolloutStuck checks whether a new version of an application is not being rolled out, either because instances of
// the new version cannot be created, or because the new version did not become ready within the allowed time.
func (w Workload) rolloutStuck(app *nais_io_v1alpha1.Application, cluster, env string) *Issue {
	if w.DeploymentWatcher == nil {
		return nil
	}
	deployment, err := w.DeploymentWatcher.Get(cluster, app.GetNamespace(), app.GetName())
	if err != nil {
		return nil
	}
	return rolloutStuckIssue(app, env, deployment)
}

func rolloutStuckIssue(app *nais_io_v1alpha1.Application, env string, deployment *appsv1.Deployment) *Issue {
	var reason issue.ApplicationRolloutStuckReason
	var message string
	for _, c := range deployment.Status.Conditions {
		if c.Type == appsv1.DeploymentReplicaFailure && c.Status == v1.ConditionTrue {
			reason = issue.ApplicationRolloutStuckReasonInstanceCreationFailed
			message = "The new version of the application cannot be rolled out. " + translate.InstanceCreationFailure(c.Message)
			break
		}
		if c.Type == appsv1.DeploymentProgressing && c.Status == v1.ConditionFalse && c.Reason == "ProgressDeadlineExceeded" {
			reason = issue.ApplicationRolloutStuckReasonTimedOut
			message = "The new version of the application did not become ready within the allowed time. Check the instances of the new version for problems."
		}
	}
	if reason == "" {
		return nil
	}

	return &Issue{
		IssueType:    issue.IssueTypeApplicationRolloutStuck,
		ResourceName: app.Name,
		ResourceType: issue.ResourceTypeApplication,
		Team:         app.GetNamespace(),
		Env:          env,
		Severity:     issue.SeverityWarning,
		Message:      message,
		IssueDetails: issue.ApplicationRolloutStuckIssueDetails{Reason: reason},
	}
}
