package event

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nais/api/internal/kubernetes/event/eventsql"
)

func TestToGraphEvent(t *testing.T) {
	t.Run("autoscaling", func(t *testing.T) {
		event, err := toGraphEvent(&eventsql.K8sEvent{Reason: "SuccessfulRescale", InvolvedKind: "HorizontalPodAutoscaler", Data: []byte(`{"newSize":"3","direction":"up","target":"above"}`)})
		if err != nil {
			t.Fatal(err)
		}
		want := &WorkloadSuccessfulRescaleEvent{
			WorkloadEventBase: WorkloadEventBase{Message: "Application scaled"},
			NewSize:           3,
			Direction:         WorkloadSuccessfulRescaleEventDirectionUp,
		}
		if diff := cmp.Diff(want, event); diff != "" {
			t.Errorf("unexpected event (-want +got):\n%s", diff)
		}
	})
	t.Run("restart", func(t *testing.T) {
		event, err := toGraphEvent(&eventsql.K8sEvent{Reason: "Killing", Data: []byte(`{"container":"app","reason":"liveness_probe_failed","workloadKind":"Application","workloadName":"app"}`)})
		if err != nil {
			t.Fatal(err)
		}
		want := &WorkloadKillingEvent{
			WorkloadEventBase: WorkloadEventBase{Message: "Instance restarted"},
			Container:         "app",
			Reason:            "liveness_probe_failed",
		}
		if diff := cmp.Diff(want, event); diff != "" {
			t.Errorf("unexpected event (-want +got):\n%s", diff)
		}
	})
	t.Run("unknown reason", func(t *testing.T) {
		_, err := toGraphEvent(&eventsql.K8sEvent{Reason: "Other", Data: []byte(`{}`)})
		if err == nil || !strings.Contains(err.Error(), "unsupported workload event reason") {
			t.Errorf("got error %v, want unsupported workload event reason", err)
		}
	})
	t.Run("reason determines type", func(t *testing.T) {
		_, err := toGraphEvent(&eventsql.K8sEvent{Reason: "Other", InvolvedKind: "HorizontalPodAutoscaler", Data: []byte(`{}`)})
		if err == nil || !strings.Contains(err.Error(), "unsupported workload event reason") {
			t.Errorf("got error %v, want unsupported workload event reason", err)
		}
	})
	t.Run("invalid scaling data", func(t *testing.T) {
		_, err := toGraphEvent(&eventsql.K8sEvent{Reason: "SuccessfulRescale", Data: []byte(`{"newSize":"invalid"}`)})
		if err == nil {
			t.Error("expected an error for invalid scaling data")
		}
	})
	t.Run("missing direction", func(t *testing.T) {
		_, err := toGraphEvent(&eventsql.K8sEvent{Reason: "SuccessfulRescale", Data: []byte(`{"newSize":"3"}`)})
		if err == nil || !strings.Contains(err.Error(), "invalid workload rescale direction") {
			t.Errorf("got error %v, want invalid workload rescale direction", err)
		}
	})
	t.Run("invalid data", func(t *testing.T) {
		_, err := toGraphEvent(&eventsql.K8sEvent{Data: []byte(`invalid`)})
		if err == nil {
			t.Error("expected an error for invalid data")
		}
	})
}
