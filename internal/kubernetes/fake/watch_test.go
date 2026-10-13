package fake_test

import (
	"context"
	"testing"
	"time"

	"github.com/nais/api/internal/kubernetes"
	"github.com/nais/api/internal/kubernetes/fake"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
)

func TestWatchLabelSelectorMembership(t *testing.T) {
	for _, tc := range []struct {
		name     string
		existing bool
		ignored  bool
	}{
		{name: "existing matching bucket", existing: true},
		{name: "existing ignored bucket", existing: true, ignored: true},
		{name: "new matching bucket"},
		{name: "new ignored bucket", ignored: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			scheme, err := kubernetes.NewScheme()
			if err != nil {
				t.Fatal(err)
			}
			client := fake.NewDynamicClient(scheme)
			gvr := schema.GroupVersionResource{Group: "storage.cnrm.cloud.google.com", Version: "v1beta1", Resource: "storagebuckets"}
			resource := client.Resource(gvr).Namespace("bucket-team")
			ctx := context.Background()
			bucket := &unstructured.Unstructured{Object: map[string]any{
				"apiVersion": "storage.cnrm.cloud.google.com/v1beta1",
				"kind":       "StorageBucket",
				"metadata":   map[string]any{"name": "assets", "namespace": "bucket-team"},
			}}
			if tc.ignored {
				bucket.SetLabels(map[string]string{"postgres.nais.io/name": "orders"})
			}
			create := func() {
				t.Helper()
				if _, err := resource.Create(ctx, bucket, metav1.CreateOptions{}); err != nil {
					t.Fatal(err)
				}
			}
			if tc.existing {
				create()
			}
			stream, err := resource.Watch(ctx, metav1.ListOptions{LabelSelector: "!postgres.nais.io/name"})
			if err != nil {
				t.Fatal(err)
			}
			defer stream.Stop()
			expect := func(eventType watch.EventType, revision string) {
				t.Helper()
				select {
				case event, ok := <-stream.ResultChan():
					if !ok {
						t.Fatal("watch closed unexpectedly")
					}
					if event.Type != eventType {
						t.Fatalf("expected %s, got %s: %v", eventType, event.Type, event.Object)
					}
					obj, ok := event.Object.(*unstructured.Unstructured)
					if !ok {
						t.Fatalf("expected unstructured bucket, got %T", event.Object)
					}
					if obj.GetName() != "assets" || obj.GetNamespace() != "bucket-team" || obj.GetResourceVersion() != revision {
						t.Fatalf("unexpected event object: %v", obj)
					}
					if _, ignored := obj.GetLabels()["postgres.nais.io/name"]; ignored {
						t.Fatal("event must contain a matching bucket")
					}
				case <-time.After(5 * time.Second):
					t.Fatalf("timed out waiting for %s", eventType)
				}
			}
			if !tc.existing {
				create()
				if !tc.ignored {
					expect(watch.Added, "")
				}
			}
			update := func(ignored bool, revision string) {
				t.Helper()
				bucket.SetLabels(nil)
				if ignored {
					bucket.SetLabels(map[string]string{"postgres.nais.io/name": "orders"})
				}
				bucket.SetResourceVersion(revision)
				if _, err := resource.Update(ctx, bucket, metav1.UpdateOptions{}); err != nil {
					t.Fatal(err)
				}
			}
			if tc.ignored {
				update(false, "1")
				expect(watch.Added, "1")
			}
			update(false, "2")
			expect(watch.Modified, "2")
			update(true, "3")
			expect(watch.Deleted, "3")
			update(true, "4")
			update(false, "5")
			expect(watch.Added, "5")
			if err := resource.Delete(ctx, bucket.GetName(), metav1.DeleteOptions{}); err != nil {
				t.Fatal(err)
			}
			expect(watch.Deleted, "5")
		})
	}
}
