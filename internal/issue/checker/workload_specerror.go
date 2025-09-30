package checker

import (
	"github.com/nais/api/internal/issue"
	libevents "github.com/nais/liberator/pkg/events"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (w Workload) specErrors(wl NaisWorkload, env string, resourceType issue.ResourceType) *Issue {
	if wl == nil || wl.GetStatus() == nil || wl.GetStatus().Conditions == nil {
		return nil
	}

	condition, ok := w.condition(*wl.GetStatus().Conditions)
	if !ok {
		return nil
	}

	switch condition.Reason {
	case libevents.FailedGenerate:
		return &Issue{
			IssueType:    issue.IssueTypeInvalidSpec,
			ResourceName: wl.GetName(),
			ResourceType: resourceType,
			Team:         wl.GetNamespace(),
			Env:          env,
			Severity:     issue.SeverityCritical,
			Message:      condition.Message,
		}

	case libevents.FailedSynchronization:
		return &Issue{
			IssueType:    issue.IssueTypeFailedSynchronization,
			ResourceName: wl.GetName(),
			ResourceType: resourceType,
			Team:         wl.GetNamespace(),
			Env:          env,
			Severity:     issue.SeverityWarning,
			Message:      condition.Message,
		}
	}

	return nil
}

func (w Workload) condition(conditions []metav1.Condition) (metav1.Condition, bool) {
	for _, condition := range conditions {
		if condition.Type == "SynchronizationState" {
			return condition, true
		}
	}
	return metav1.Condition{}, false
}
