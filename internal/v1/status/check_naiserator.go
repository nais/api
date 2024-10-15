package status

import (
	"context"

	"github.com/nais/api/internal/v1/workload"
	libevents "github.com/nais/liberator/pkg/events"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type checkNaiserator struct{}

func (c checkNaiserator) Run(ctx context.Context, w workload.Workload) ([]WorkloadStatusError, WorkloadState) {
	s := c.run(ctx, w)
	if s == nil {
		return nil, WorkloadStateNais
	}
	return []WorkloadStatusError{s}, WorkloadStateNotNais
}

func (c checkNaiserator) run(_ context.Context, w workload.Workload) WorkloadStatusError {
	condition, ok := c.condition(w.GetConditions())
	if !ok {
		return nil
	}

	switch condition.Reason {
	// A FailedGenerate error is almost always because of invalid yaml.
	case libevents.FailedGenerate:
		return &WorkloadStatusInvalidNaisYaml{
			Level:  WorkloadStatusErrorLevelError,
			Detail: condition.Message,
		}

	case libevents.FailedSynchronization:
		return &WorkloadStatusSynchronizationFailing{
			Level:  WorkloadStatusErrorLevelError,
			Detail: condition.Message,
		}

		// TODO(thokra): We already have a check for failed instances on applications.
		// Is it correct to have this check here?
		// case libevents.Synchronized:
		// 	if _, ok := w.(*job.Job); !ok {
		// 		// This should not be done for jobs
		// 		return nil
		// 	}
		// 	// TODO(thokra): Possibly move this into a resolver
		// 	// names := []string{}
		// 	// for _, instance := range failingInstances {
		// 	// 	names = append(names, instance.Name)
		// 	// }
		// 	return &WorkloadStatusNewInstancesFailing{
		// 		Level: WorkloadStatusErrorLevelWarning,
		// 		// FailingInstances: names,
		// 	}
	}
	return nil
}

func (checkNaiserator) Supports(w workload.Workload) bool {
	return true
}

func (checkNaiserator) condition(conditions []metav1.Condition) (metav1.Condition, bool) {
	for _, condition := range conditions {
		if condition.Type == "SynchronizationState" {
			return condition, true
		}
	}
	return metav1.Condition{}, false
}
