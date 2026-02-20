package postgres

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPostgresStateFromConditions(t *testing.T) {
	tests := []struct {
		name       string
		conditions []metav1.Condition
		want       PostgresInstanceState
	}{
		{
			name: "degraded when degraded is true",
			conditions: []metav1.Condition{
				{Type: postgresConditionTypeAvailable, Status: metav1.ConditionTrue},
				{Type: postgresConditionTypeDegraded, Status: metav1.ConditionTrue},
			},
			want: PostgresInstanceStateDegraded,
		},
		{
			name: "progressing when progressing is true",
			conditions: []metav1.Condition{
				{Type: postgresConditionTypeProgressing, Status: metav1.ConditionTrue},
			},
			want: PostgresInstanceStateProgressing,
		},
		{
			name: "available when available is true",
			conditions: []metav1.Condition{
				{Type: postgresConditionTypeAvailable, Status: metav1.ConditionTrue},
			},
			want: PostgresInstanceStateAvailable,
		},
		{
			name: "available when no recognized true condition",
			conditions: []metav1.Condition{
				{Type: postgresConditionTypeAvailable, Status: metav1.ConditionFalse},
				{Type: postgresConditionTypeProgressing, Status: metav1.ConditionFalse},
			},
			want: PostgresInstanceStateAvailable,
		},
		{
			name: "available when no conditions",
			want: PostgresInstanceStateAvailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := postgresStateFromConditions(tt.conditions)
			if got != tt.want {
				t.Errorf("postgresStateFromConditions() = %q, want %q", got, tt.want)
			}
		})
	}
}
