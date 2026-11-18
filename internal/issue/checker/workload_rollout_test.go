package checker

import (
	"testing"

	"github.com/nais/api/internal/issue"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
)

func TestRolloutStuckIssue(t *testing.T) {
	app := makeApp("myapp")

	tests := []struct {
		name       string
		conditions []appsv1.DeploymentCondition
		wantReason issue.ApplicationRolloutStuckReason
	}{
		{
			name: "progressing deployment has no issue",
			conditions: []appsv1.DeploymentCondition{
				{Type: appsv1.DeploymentProgressing, Status: v1.ConditionTrue, Reason: "NewReplicaSetAvailable"},
			},
		},
		{
			name: "progress deadline exceeded",
			conditions: []appsv1.DeploymentCondition{
				{Type: appsv1.DeploymentProgressing, Status: v1.ConditionFalse, Reason: "ProgressDeadlineExceeded"},
			},
			wantReason: issue.ApplicationRolloutStuckReasonTimedOut,
		},
		{
			name: "replica failure takes precedence over deadline exceeded",
			conditions: []appsv1.DeploymentCondition{
				{Type: appsv1.DeploymentProgressing, Status: v1.ConditionFalse, Reason: "ProgressDeadlineExceeded"},
				{Type: appsv1.DeploymentReplicaFailure, Status: v1.ConditionTrue, Reason: "FailedCreate", Message: `pods "myapp-abc" is forbidden: exceeded quota: compute`},
			},
			wantReason: issue.ApplicationRolloutStuckReasonInstanceCreationFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &appsv1.Deployment{Status: appsv1.DeploymentStatus{Conditions: tt.conditions}}
			got := rolloutStuckIssue(app, "dev", d)
			if tt.wantReason == "" {
				if got != nil {
					t.Fatalf("expected no issue, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("expected issue, got nil")
			}
			if got.Severity != issue.SeverityWarning {
				t.Errorf("severity = %v, want WARNING", got.Severity)
			}
			details, ok := got.IssueDetails.(issue.ApplicationRolloutStuckIssueDetails)
			if !ok {
				t.Fatalf("unexpected details type %T", got.IssueDetails)
			}
			if details.Reason != tt.wantReason {
				t.Errorf("reason = %v, want %v", details.Reason, tt.wantReason)
			}
			if got.Message == "" {
				t.Error("expected message")
			}
		})
	}
}
