package checker

import (
	"fmt"
	"testing"
	"time"

	"github.com/nais/api/internal/issue"
	nais_io_v1alpha1 "github.com/nais/liberator/pkg/apis/nais.io/v1alpha1"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func makeApp(name string) *nais_io_v1alpha1.Application {
	return &nais_io_v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "team1",
		},
	}
}

func makePod(appName string, restartCount int32, finishedAt time.Time, reason string) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-pod", appName),
			Namespace: "team1",
		},
		Status: v1.PodStatus{
			ContainerStatuses: []v1.ContainerStatus{
				{
					Name:         appName,
					RestartCount: restartCount,
					LastTerminationState: v1.ContainerState{
						Terminated: &v1.ContainerStateTerminated{
							FinishedAt: metav1.NewTime(finishedAt),
							Reason:     reason,
						},
					},
				},
			},
		},
	}
}

func TestRestartLoop(t *testing.T) {
	w := Workload{log: logrus.New()}
	app := makeApp("myapp")
	now := time.Now()

	tests := []struct {
		name         string
		pods         []*v1.Pod
		wantNil      bool
		wantSeverity issue.Severity
		wantReason   string
	}{
		{
			name:    "no pods returns no issue",
			pods:    nil,
			wantNil: true,
		},
		{
			name: "restarts but last termination was long ago returns no issue",
			pods: []*v1.Pod{
				makePod("myapp", 5, now.Add(-40*time.Minute), "OOMKilled"),
			},
			wantNil: true,
		},
		{
			name: "warning threshold: >=3 restarts within 30 minutes",
			pods: []*v1.Pod{
				makePod("myapp", 3, now.Add(-20*time.Minute), "OOMKilled"),
			},
			wantNil:      false,
			wantSeverity: issue.SeverityWarning,
			wantReason:   "OOMKilled",
		},
		{
			name: "critical threshold: >=10 restarts within 10 minutes",
			pods: []*v1.Pod{
				makePod("myapp", 10, now.Add(-5*time.Minute), "Error"),
			},
			wantNil:      false,
			wantSeverity: issue.SeverityCritical,
			wantReason:   "Error",
		},
		{
			name: "critical takes precedence over warning when multiple pods",
			pods: []*v1.Pod{
				makePod("myapp", 3, now.Add(-20*time.Minute), "OOMKilled"),
				makePod("myapp", 10, now.Add(-5*time.Minute), "Error"),
			},
			wantNil:      false,
			wantSeverity: issue.SeverityCritical,
			wantReason:   "Error",
		},
		{
			name: "critical takes precedence over warning even with higher warning restart count",
			pods: []*v1.Pod{
				makePod("myapp", 10, now.Add(-5*time.Minute), "Error"),
				makePod("myapp", 15, now.Add(-25*time.Minute), "OOMKilled"),
			},
			wantNil:      false,
			wantSeverity: issue.SeverityCritical,
			wantReason:   "Error",
		},
		{
			name: "empty reason falls back to exit code",
			pods: []*v1.Pod{
				func() *v1.Pod {
					p := makePod("myapp", 10, now.Add(-5*time.Minute), "")
					p.Status.ContainerStatuses[0].LastTerminationState.Terminated.ExitCode = 137
					return p
				}(),
			},
			wantNil:      false,
			wantSeverity: issue.SeverityCritical,
			wantReason:   "ExitCode:137",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := w.restartLoop(app, tc.pods, "team1", "dev")
			if tc.wantNil {
				if got != nil {
					t.Fatalf("expected nil issue, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("expected non-nil issue, got nil")
			}
			if got.Severity != tc.wantSeverity {
				t.Errorf("severity: want %v, got %v", tc.wantSeverity, got.Severity)
			}
			details, ok := got.IssueDetails.(issue.RestartLoopIssueDetails)
			if !ok {
				t.Fatalf("expected RestartLoopIssueDetails, got %T", got.IssueDetails)
			}
			if details.LastExitReason != tc.wantReason {
				t.Errorf("lastExitReason: want %q, got %q", tc.wantReason, details.LastExitReason)
			}
		})
	}
}
