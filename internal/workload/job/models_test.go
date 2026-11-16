package job

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestJobRunInstanceStatus(t *testing.T) {
	pod := func(conditions []corev1.PodCondition, statuses ...corev1.ContainerStatus) *corev1.Pod {
		return &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "myjob-abc",
				Namespace: "team1",
				Labels:    map[string]string{"app": "myjob"},
			},
			Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "myjob", Image: "example.com/myjob:1"}}},
			Status: corev1.PodStatus{
				Conditions:        conditions,
				ContainerStatuses: statuses,
			},
		}
	}

	tests := []struct {
		name        string
		pod         *corev1.Pod
		wantState   JobRunInstanceState
		wantMessage string
	}{
		{
			name: "completed successfully",
			pod: pod(nil, corev1.ContainerStatus{
				Name:  "myjob",
				State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{Reason: "Completed"}},
			}),
			wantState:   JobRunInstanceStateSucceeded,
			wantMessage: "Completed successfully.",
		},
		{
			name: "ran out of memory",
			pod: pod(nil, corev1.ContainerStatus{
				Name:  "myjob",
				State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{Reason: "OOMKilled", ExitCode: 137}},
			}),
			wantState:   JobRunInstanceStateFailing,
			wantMessage: "Instance ran out of memory. Consider increasing the memory limit.",
		},
		{
			name: "running with failing component does not mention traffic",
			pod: pod(nil,
				corev1.ContainerStatus{
					Name:  "myjob",
					State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
				},
				corev1.ContainerStatus{
					Name:  "sidecar",
					State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}},
				},
			),
			wantState:   JobRunInstanceStateRunning,
			wantMessage: "Running. A component added by the platform keeps crashing.",
		},
		{
			name: "pending when it cannot be placed",
			pod: pod([]corev1.PodCondition{{
				Type:    corev1.PodScheduled,
				Status:  corev1.ConditionFalse,
				Reason:  corev1.PodReasonUnschedulable,
				Message: "0/3 nodes are available: 3 Insufficient memory.",
			}}),
			wantState: JobRunInstanceStatePending,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toGraphJobRunInstance(tt.pod, "dev").Status()
			if got.State != tt.wantState {
				t.Errorf("state = %v, want %v", got.State, tt.wantState)
			}
			if tt.wantMessage != "" && got.Message != tt.wantMessage {
				t.Errorf("message = %q, want %q", got.Message, tt.wantMessage)
			}
		})
	}
}
