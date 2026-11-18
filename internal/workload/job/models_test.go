package job

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestJobRunInstanceStatusLastExit(t *testing.T) {
	firstExit := metav1.NewTime(time.Date(2026, time.November, 18, 8, 0, 0, 0, time.UTC))
	lastExit := metav1.NewTime(firstExit.Add(time.Minute))
	failure := &corev1.ContainerStateTerminated{Reason: "Error", ExitCode: 42, FinishedAt: firstExit}
	success := &corev1.ContainerStateTerminated{Reason: "Completed", FinishedAt: lastExit}

	for _, tt := range []struct {
		name     string
		state    corev1.ContainerState
		previous *corev1.ContainerStateTerminated
		want     *corev1.ContainerStateTerminated
	}{
		{
			name:  "first failure",
			state: corev1.ContainerState{Terminated: failure},
			want:  failure,
		},
		{
			name:     "success after failed restart",
			state:    corev1.ContainerState{Terminated: success},
			previous: failure,
			want:     success,
		},
		{
			name:     "running after failed restart",
			state:    corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
			previous: failure,
			want:     failure,
		},
		{
			name:     "waiting after failure",
			state:    corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}},
			previous: failure,
			want:     failure,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "myjob"}},
				Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{{
					Name:                 "myjob",
					State:                tt.state,
					LastTerminationState: corev1.ContainerState{Terminated: tt.previous},
				}}},
			}
			got := toGraphJobRunInstance(pod, "dev").Status()
			if got.LastExitReason == nil || *got.LastExitReason != tt.want.Reason {
				t.Errorf("last exit reason = %v, want %q", got.LastExitReason, tt.want.Reason)
			}
			if got.LastExitCode == nil || *got.LastExitCode != int(tt.want.ExitCode) {
				t.Errorf("last exit code = %v, want %d", got.LastExitCode, tt.want.ExitCode)
			}
			if got.LastExitTimestamp == nil || !got.LastExitTimestamp.Equal(tt.want.FinishedAt.Time) {
				t.Errorf("last exit timestamp = %v, want %v", got.LastExitTimestamp, tt.want.FinishedAt.Time)
			}
		})
	}
}

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
