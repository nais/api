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

func TestApplicationInstanceStatusComponentProblem(t *testing.T) {
	crashing := corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}}

	for _, test := range []struct {
		name        string
		init        []corev1.ContainerStatus
		sidecars    []corev1.ContainerStatus
		app         corev1.ContainerState
		appReady    bool
		wantState   ApplicationInstanceState
		wantReady   bool
		wantMessage string
	}{
		{
			name:        "failing init container blocks start",
			init:        []corev1.ContainerStatus{{Name: "init", State: crashing}},
			app:         corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "PodInitializing"}},
			wantState:   ApplicationInstanceStateFailing,
			wantMessage: "A component added by the platform keeps crashing.",
		},
		{
			name: "missing configuration for init container",
			init: []corev1.ContainerStatus{{Name: "init", State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{
				Reason: "CreateContainerConfigError", Message: `secret "db" not found`,
			}}}},
			wantState:   ApplicationInstanceStateFailing,
			wantMessage: `A component added by the platform is missing configuration. Secret "db" was not found.`,
		},
		{
			name: "failing sidecar stops traffic to running app",
			sidecars: []corev1.ContainerStatus{{Name: "sidecar", State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{
				Reason: "OOMKilled", ExitCode: 137,
			}}}},
			app:         corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
			appReady:    true,
			wantState:   ApplicationInstanceStateRunning,
			wantMessage: "Running, but not receiving traffic. A component added by the platform ran out of memory.",
		},
		{
			name:        "application problem takes precedence",
			sidecars:    []corev1.ContainerStatus{{Name: "sidecar", State: crashing}},
			app:         corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ErrImagePull"}},
			wantState:   ApplicationInstanceStateFailing,
			wantMessage: "Failed to download container image. Check that the image exists and is accessible.",
		},
		{
			name: "healthy components",
			init: []corev1.ContainerStatus{{Name: "init", State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{
				Reason: "Completed",
			}}}},
			sidecars:    []corev1.ContainerStatus{{Name: "sidecar", State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}}},
			app:         corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
			appReady:    true,
			wantState:   ApplicationInstanceStateRunning,
			wantReady:   true,
			wantMessage: "Running and ready.",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			pod := &corev1.Pod{
				Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "app", Image: "image"}}},
				Status: corev1.PodStatus{
					InitContainerStatuses: test.init,
					ContainerStatuses:     append([]corev1.ContainerStatus{{Name: "app", State: test.app, Ready: test.appReady}}, test.sidecars...),
				},
			}
			status := ToApplicationInstance(pod, "team", "dev", "app").Status()
			if status.State != test.wantState {
				t.Errorf("got state %q, want %q", status.State, test.wantState)
			}
			if status.Ready != test.wantReady {
				t.Errorf("got ready %v, want %v", status.Ready, test.wantReady)
			}
			if status.Message != test.wantMessage {
				t.Errorf("got message %q, want %q", status.Message, test.wantMessage)
			}
		})
	}
}
