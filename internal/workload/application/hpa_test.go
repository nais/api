package application

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestToScalingStatus(t *testing.T) {
	cond := func(typ autoscalingv2.HorizontalPodAutoscalerConditionType, status corev1.ConditionStatus, reason string) autoscalingv2.HorizontalPodAutoscalerCondition {
		return autoscalingv2.HorizontalPodAutoscalerCondition{Type: typ, Status: status, Reason: reason}
	}
	healthy := []autoscalingv2.HorizontalPodAutoscalerCondition{
		cond(autoscalingv2.AbleToScale, corev1.ConditionTrue, "ReadyForNewScale"),
		cond(autoscalingv2.ScalingActive, corev1.ConditionTrue, "ValidMetricFound"),
		cond(autoscalingv2.ScalingLimited, corev1.ConditionFalse, "DesiredWithinRange"),
	}

	tests := []struct {
		name       string
		conditions []autoscalingv2.HorizontalPodAutoscalerCondition
		want       ApplicationScalingState
	}{
		{name: "healthy", conditions: healthy, want: ApplicationScalingStateActive},
		{
			name: "held at minimum is normal",
			conditions: []autoscalingv2.HorizontalPodAutoscalerCondition{
				cond(autoscalingv2.ScalingLimited, corev1.ConditionTrue, "TooFewReplicas"),
			},
			want: ApplicationScalingStateActive,
		},
		{
			name: "limited by maximum",
			conditions: []autoscalingv2.HorizontalPodAutoscalerCondition{
				cond(autoscalingv2.ScalingLimited, corev1.ConditionTrue, "TooManyReplicas"),
			},
			want: ApplicationScalingStateLimitedByMaximum,
		},
		{
			name: "metrics unavailable takes precedence over limited",
			conditions: []autoscalingv2.HorizontalPodAutoscalerCondition{
				cond(autoscalingv2.ScalingActive, corev1.ConditionFalse, "FailedGetResourceMetric"),
				cond(autoscalingv2.ScalingLimited, corev1.ConditionTrue, "TooManyReplicas"),
			},
			want: ApplicationScalingStateMetricsUnavailable,
		},
		{
			name: "scaled to zero",
			conditions: []autoscalingv2.HorizontalPodAutoscalerCondition{
				cond(autoscalingv2.ScalingActive, corev1.ConditionFalse, "ScalingDisabled"),
			},
			want: ApplicationScalingStateDisabled,
		},
		{
			name: "unable to scale takes precedence",
			conditions: []autoscalingv2.HorizontalPodAutoscalerCondition{
				cond(autoscalingv2.AbleToScale, corev1.ConditionFalse, "FailedUpdateScale"),
				cond(autoscalingv2.ScalingActive, corev1.ConditionFalse, "FailedGetResourceMetric"),
			},
			want: ApplicationScalingStateUnableToScale,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toScalingStatus(&autoscalingv2.HorizontalPodAutoscaler{
				Status: autoscalingv2.HorizontalPodAutoscalerStatus{
					CurrentReplicas: 2,
					DesiredReplicas: 3,
					Conditions:      tt.conditions,
				},
			})
			if got.State != tt.want {
				t.Errorf("state = %v, want %v", got.State, tt.want)
			}
			if got.CurrentInstances != 2 || got.DesiredInstances != 3 {
				t.Errorf("instances = %d/%d, want 2/3", got.CurrentInstances, got.DesiredInstances)
			}
			if got.Message == "" {
				t.Error("expected message")
			}
		})
	}
}

func TestTransformHorizontalPodAutoscaler(t *testing.T) {
	in := &autoscalingv2.HorizontalPodAutoscaler{}
	in.SetName("myapp")
	in.SetLabels(map[string]string{"app": "myapp", "team": "team1", "other": "x"})
	in.SetAnnotations(map[string]string{"a": "b"})
	in.Spec.MaxReplicas = 4
	in.Status = autoscalingv2.HorizontalPodAutoscalerStatus{
		CurrentReplicas: 2,
		DesiredReplicas: 4,
		Conditions: []autoscalingv2.HorizontalPodAutoscalerCondition{
			{Type: autoscalingv2.ScalingLimited, Status: corev1.ConditionTrue, Reason: "TooManyReplicas", Message: "the desired replica count is more than the maximum replica count"},
		},
	}
	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(in)
	if err != nil {
		t.Fatal(err)
	}

	out, err := transformHorizontalPodAutoscaler(&unstructured.Unstructured{Object: obj})
	if err != nil {
		t.Fatal(err)
	}

	var got autoscalingv2.HorizontalPodAutoscaler
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(out.(*unstructured.Unstructured).Object, &got); err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(map[string]string{"app": "myapp", "team": "team1"}, got.GetLabels()); diff != "" {
		t.Errorf("labels diff (-want +got):\n%s", diff)
	}
	if got.GetAnnotations() != nil || got.Spec.MaxReplicas != 0 {
		t.Error("expected annotations and spec to be removed")
	}
	if diff := cmp.Diff(in.Status, got.Status); diff != "" {
		t.Errorf("status diff (-want +got):\n%s", diff)
	}
}
