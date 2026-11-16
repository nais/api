package event

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/nais/api/internal/kubernetes/event/eventsql"
	corev1 "k8s.io/api/core/v1"
	eventv1 "k8s.io/api/events/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestTranslateWarning(t *testing.T) {
	for _, test := range []struct {
		reason, note string
		wantKind     WorkloadWarningEventKind
		wantMessage  string
		wantSkip     bool
	}{
		{
			reason: "FailedScheduling", note: "0/12 nodes are available: 3 Insufficient cpu, 4 Insufficient memory. preemption: 0/12 nodes are available.",
			wantKind: WorkloadWarningEventKindPlacementFailed, wantMessage: "Not enough CPU and memory available to start the instance. Consider lowering the requested resources.",
		},
		{
			reason: "FailedScheduling", note: "0/12 nodes are available: 12 node(s) didn't match Pod's node affinity/selector.",
			wantKind: WorkloadWarningEventKindPlacementFailed, wantMessage: "No suitable server was found to run the instance.",
		},
		{
			reason: "FailedMount", note: `MountVolume.SetUp failed for volume "my-secret" : secret "my-secret" not found`,
			wantKind: WorkloadWarningEventKindMountFailed, wantMessage: `Failed to mount files into the instance. Secret "my-secret" was not found.`,
		},
		{
			reason: "FailedMount", note: `MountVolume.SetUp failed for volume "cfg" : configmap "my-config" not found`,
			wantKind: WorkloadWarningEventKindMountFailed, wantMessage: `Failed to mount files into the instance. Config "my-config" was not found.`,
		},
		{
			reason: "Failed", note: `Failed to pull image "europe-north1-docker.pkg.dev/nais/app:1": not found`,
			wantKind: WorkloadWarningEventKindImagePullFailed, wantMessage: "Failed to download the container image. Check that the image exists and is accessible.",
		},
		{reason: "Failed", note: "Error: ImagePullBackOff", wantSkip: true},
		{
			reason: "Failed", note: "Error: couldn't find key DB_PASSWORD in Secret team/db-secret",
			wantKind: WorkloadWarningEventKindStartFailed, wantMessage: `The instance failed to start. Key "DB_PASSWORD" was not found in secret "db-secret".`,
		},
		{
			reason: "Failed", note: `Error: secret "missing" not found`,
			wantKind: WorkloadWarningEventKindStartFailed, wantMessage: `The instance failed to start. Secret "missing" was not found.`,
		},
		{
			reason: "BackOff", note: "Back-off restarting failed container app in pod app-abc_team(uid)",
			wantKind: WorkloadWarningEventKindCrashLoop, wantMessage: "The instance keeps crashing and is restarted with increasing delay. Check the logs for details.",
		},
		{reason: "BackOff", note: `Back-off pulling image "image"`, wantSkip: true},
		{
			reason: "Unhealthy", note: "Readiness probe failed: HTTP probe failed with statuscode: 503",
			wantKind: WorkloadWarningEventKindHealthCheckFailed, wantMessage: "Readiness check failed: responded with status code 503. The instance does not receive traffic while failing.",
		},
		{
			reason: "Unhealthy", note: `Liveness probe failed: Get "http://10.0.0.1:8080/isalive": dial tcp 10.0.0.1:8080: connect: connection refused`,
			wantKind: WorkloadWarningEventKindHealthCheckFailed, wantMessage: "Liveness check failed: the connection was refused. The instance is restarted if it keeps failing.",
		},
		{
			reason: "Unhealthy", note: `Startup probe failed: Get "http://10.0.0.1:8080/isready": context deadline exceeded (Client.Timeout exceeded while awaiting headers)`,
			wantKind: WorkloadWarningEventKindHealthCheckFailed, wantMessage: "Startup check failed: timed out. The instance is restarted if it does not start in time.",
		},
		{
			reason: "Evicted", note: "The node was low on resource: memory. Threshold quantity: 100Mi, available: 50Mi.",
			wantKind: WorkloadWarningEventKindInstanceStopped, wantMessage: "The instance was stopped because the server ran low on memory.",
		},
		{
			reason: "Evicted", note: "Pod ephemeral local storage usage exceeds the total limit of containers 1Gi.",
			wantKind: WorkloadWarningEventKindInstanceStopped, wantMessage: "The instance was stopped because it used more disk space than allowed.",
		},
		{
			reason: "FailedCreate", note: `Error creating: pods "app-abc" is forbidden: exceeded quota: team-quota, requested: cpu=2`,
			wantKind: WorkloadWarningEventKindInstanceCreationFailed, wantMessage: "New instances could not be created because the team's resource quota is exceeded.",
		},
		{
			reason: "FailedCreate", note: `Error creating: admission webhook "validate.kyverno.svc" denied the request: policy violated`,
			wantKind: WorkloadWarningEventKindInstanceCreationFailed, wantMessage: "New instances were rejected by a platform policy.",
		},
		{
			reason: "FailedGetResourceMetric", note: "failed to get cpu utilization: unable to get metrics for resource cpu",
			wantKind: WorkloadWarningEventKindScalingFailed, wantMessage: "Automatic scaling could not read the metrics it needs, so the number of instances is not adjusted.",
		},
		{
			reason: "BackoffLimitExceeded", note: "Job has reached the specified backoff limit",
			wantKind: WorkloadWarningEventKindRunFailed, wantMessage: "The run failed too many times and will not be retried.",
		},
		{reason: "SomethingElse", note: "note", wantSkip: true},
	} {
		t.Run(test.reason+": "+test.note, func(t *testing.T) {
			kind, message, ok := translateWarning(test.reason, test.note)
			if ok == test.wantSkip {
				t.Fatalf("got ok %v, want %v", ok, !test.wantSkip)
			}
			if kind != test.wantKind {
				t.Errorf("got kind %q, want %q", kind, test.wantKind)
			}
			if message != test.wantMessage {
				t.Errorf("got message %q, want %q", message, test.wantMessage)
			}
		})
	}
}

func TestWarningData(t *testing.T) {
	first := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)

	t.Run("pod event with series", func(t *testing.T) {
		data, ok := warningData(&eventv1.Event{
			Reason:                   "Unhealthy",
			Note:                     "Readiness probe failed: HTTP probe failed with statuscode: 500",
			Regarding:                corev1.ObjectReference{Kind: "Pod", Name: "app-abc"},
			Series:                   &eventv1.EventSeries{Count: 7},
			DeprecatedFirstTimestamp: metav1.NewTime(first),
		})
		if !ok {
			t.Fatal("expected event to be persisted")
		}
		want := map[string]any{
			"note":      "Readiness probe failed: HTTP probe failed with statuscode: 500",
			"count":     7,
			"firstSeen": first,
			"instance":  "app-abc",
		}
		if diff := cmp.Diff(want, data); diff != "" {
			t.Errorf("unexpected data (-want +got):\n%s", diff)
		}
	})

	t.Run("non-pod event without series", func(t *testing.T) {
		data, ok := warningData(&eventv1.Event{
			Reason:    "FailedCreate",
			Note:      "Error creating: exceeded quota",
			Regarding: corev1.ObjectReference{Kind: "ReplicaSet", Name: "app-abc"},
			EventTime: metav1.NewMicroTime(first),
		})
		if !ok {
			t.Fatal("expected event to be persisted")
		}
		want := map[string]any{"note": "Error creating: exceeded quota", "count": 1, "firstSeen": first}
		if diff := cmp.Diff(want, data); diff != "" {
			t.Errorf("unexpected data (-want +got):\n%s", diff)
		}
	})

	t.Run("skipped notes are not persisted", func(t *testing.T) {
		if _, ok := warningData(&eventv1.Event{Reason: "Failed", Note: "Error: ErrImagePull"}); ok {
			t.Error("expected event to be skipped")
		}
	})
}

func TestToGraphWarningEvent(t *testing.T) {
	triggered := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	row := func(data string) *eventsql.K8sEvent {
		r := &eventsql.K8sEvent{Reason: "BackOff", Data: []byte(data)}
		r.TriggeredAt.Time = triggered
		r.TriggeredAt.Valid = true
		return r
	}

	event, err := toGraphEvent(row(`{"note":"Back-off restarting failed container app in pod app","instance":"app-abc","count":4,"firstSeen":"2026-01-01T10:00:00Z"}`))
	if err != nil {
		t.Fatal(err)
	}
	want := &WorkloadWarningEvent{
		WorkloadEventBase: WorkloadEventBase{
			TriggeredAt: triggered,
			Message:     "The instance keeps crashing and is restarted with increasing delay. Check the logs for details.",
		},
		Kind:        WorkloadWarningEventKindCrashLoop,
		Instance:    new("app-abc"),
		Count:       4,
		FirstSeenAt: time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC),
	}
	if diff := cmp.Diff(want, event); diff != "" {
		t.Errorf("unexpected event (-want +got):\n%s", diff)
	}

	event, err = toGraphEvent(row(`{"note":"Back-off restarting failed container app in pod app"}`))
	if err != nil {
		t.Fatal(err)
	}
	got := event.(*WorkloadWarningEvent)
	if got.Count != 1 || !got.FirstSeenAt.Equal(triggered) || got.Instance != nil {
		t.Errorf("got count %d, first seen %v, instance %v; want defaults", got.Count, got.FirstSeenAt, got.Instance)
	}

	if _, err := toGraphEvent(row(`{"note":"Back-off pulling image"}`)); err == nil {
		t.Error("expected an error for an untranslatable note")
	}
}
