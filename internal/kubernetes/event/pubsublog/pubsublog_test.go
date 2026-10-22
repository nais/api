package pubsublog

import (
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nais/api/internal/activitylog/activitylogsql"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type captureAuditQuerier struct {
	activitylogsql.Querier
	entries []activitylogsql.CreateClusterAuditParams
}

func (q *captureAuditQuerier) CreateClusterAudit(_ context.Context, params activitylogsql.CreateClusterAuditParams) error {
	q.entries = append(q.entries, params)
	return nil
}

type kindResolver string

func (k kindResolver) KindsFor(resource schema.GroupVersionResource) ([]schema.GroupVersionKind, error) {
	return []schema.GroupVersionKind{resource.GroupVersion().WithKind(string(k))}, nil
}

func TestReceiveCreatesActivityLogEntry(t *testing.T) {
	for _, test := range []struct {
		name, resource, resourceName, kind, method, action string
	}{
		{"deployment", "apps/v1/namespaces/slug-1/deployments/app", "app", "Deployment", "io.k8s.apps.v1.deployments.patch", "Patch"},
		{"pod exec", "core/v1/namespaces/slug-1/pods/app-pod", "app-pod", "Pod", "io.k8s.core.v1.pods.exec.create", "Exec"},
		{"autoscaler audit", "autoscaling/v2/namespaces/slug-1/horizontalpodautoscalers/app", "app", "HorizontalPodAutoscaler", "io.k8s.autoscaling.v2.horizontalpodautoscalers.patch", "Patch"},
	} {
		t.Run(test.name, func(t *testing.T) {
			q := &captureAuditQuerier{}
			log := logrus.New()
			log.SetOutput(io.Discard)
			s, err := NewSubscriber(nil, q, map[string]watcher.KindResolver{"dev": kindResolver(test.kind)}, log)
			if err != nil {
				t.Fatal(err)
			}
			line := LogLine{Timestamp: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)}
			line.Operation.ID = uuid.New()
			line.Resource.Labels.ClusterName = "nais-dev"
			line.ProtoPayload.AuthenticationInfo.PrincipalEmail = "user@example.com"
			line.ProtoPayload.ResourceName = test.resource
			line.ProtoPayload.MethodName = test.method
			data, err := json.Marshal(line)
			if err != nil {
				t.Fatal(err)
			}
			s.receive(context.Background(), &pubsub.Message{Data: data})
			if len(q.entries) != 1 {
				t.Fatalf("got %d entries, want 1", len(q.entries))
			}
			entry := q.entries[0]
			want := activitylogsql.CreateClusterAuditParams{
				Actor:              "user@example.com",
				Namespace:          "slug-1",
				ResourceName:       test.resourceName,
				EnvironmentName:    new("dev"),
				CreatedAt:          pgtype.Timestamptz{Time: line.Timestamp, Valid: true},
				KubernetesAuditUid: line.Operation.ID,
			}
			if diff := cmp.Diff(want, entry, cmpopts.IgnoreFields(activitylogsql.CreateClusterAuditParams{}, "Data")); diff != "" {
				t.Errorf("unexpected activity log entry (-want +got):\n%s", diff)
			}
			var payload map[string]string
			if err := json.Unmarshal(entry.Data, &payload); err != nil {
				t.Fatal(err)
			}
			wantPayload := map[string]string{"action": test.action, "resourceKind": test.kind}
			if diff := cmp.Diff(wantPayload, payload); diff != "" {
				t.Errorf("unexpected payload (-want +got):\n%s", diff)
			}
		})
	}
}
