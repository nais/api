package application

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestApplicationInstanceStatusPending(t *testing.T) {
	unschedulable := corev1.PodCondition{
		Type:    corev1.PodScheduled,
		Status:  corev1.ConditionFalse,
		Reason:  corev1.PodReasonUnschedulable,
		Message: "0/3 nodes are available: 3 Insufficient memory.",
	}

	for _, test := range []struct {
		name        string
		conditions  []corev1.PodCondition
		container   corev1.ContainerState
		wantState   ApplicationInstanceState
		wantMessage string
	}{
		{
			name:        "unschedulable",
			conditions:  []corev1.PodCondition{unschedulable},
			wantState:   ApplicationInstanceStatePending,
			wantMessage: "Not enough memory available to start the instance. Consider lowering the requested resources.",
		},
		{
			name:       "unschedulable with stale running container",
			conditions: []corev1.PodCondition{unschedulable},
			container:  corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
			wantState:  ApplicationInstanceStateRunning,
		},
		{
			name:       "scheduling in progress",
			conditions: []corev1.PodCondition{{Type: corev1.PodScheduled, Status: corev1.ConditionFalse}},
			wantState:  ApplicationInstanceStateUnknown,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			pod := &corev1.Pod{
				Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "app", Image: "image"}}},
				Status: corev1.PodStatus{
					Conditions:        test.conditions,
					ContainerStatuses: []corev1.ContainerStatus{{Name: "app", State: test.container}},
				},
			}
			status := ToApplicationInstance(pod, "team", "dev", "app").Status()
			if status.State != test.wantState {
				t.Errorf("got state %q, want %q", status.State, test.wantState)
			}
			if test.wantMessage != "" && status.Message != test.wantMessage {
				t.Errorf("got message %q, want %q", status.Message, test.wantMessage)
			}
		})
	}
}
