package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/kubernetes"
	"github.com/nais/api/internal/kubernetes/fake"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/user"
	"github.com/sirupsen/logrus/hooks/test"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestPostgresUpdateKeepsOptimisticConcurrency(t *testing.T) {
	for _, tt := range []struct {
		name      string
		input     UpdatePostgresInput
		wantWrite bool
	}{
		{name: "unchanged quantities do not write", input: UpdatePostgresInput{Memory: new("1Gi")}},
		{name: "conflicting update is not retried", input: UpdatePostgresInput{CPU: new("250m")}, wantWrite: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			scheme, err := kubernetes.NewScheme()
			if err != nil {
				t.Fatal(err)
			}
			clusters, err := kubernetes.CreateClusterConfigMap("nav", []string{"dev"}, nil)
			if err != nil {
				t.Fatal(err)
			}
			log, _ := test.NewNullLogger()
			mgr, err := watcher.NewManager(scheme, clusters, log, watcher.WithClientCreator(fake.Clients(nil)))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(mgr.Stop)
			client := mgr.GetDynamicClients()["dev"].(*dynfake.FakeDynamicClient)
			gvr := schema.GroupVersionResource{Group: "nais.io", Version: "v1", Resource: "postgres"}
			resources := client.Resource(gvr).Namespace("team")
			existing, err := resources.Create(t.Context(), postgresWithNewerFields(), metav1.CreateOptions{})
			if err != nil {
				t.Fatal(err)
			}
			branchWatcher := NewPostgresBranchWatcher(t.Context(), mgr)
			postgresWatcher := NewPostgresWatcher(t.Context(), mgr)
			wait, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			if !mgr.WaitForReady(wait) {
				t.Fatal("watchers did not synchronize")
			}
			ctx := NewLoaderContext(t.Context(), branchWatcher, postgresWatcher, "", "", "nav", mgr.GetDynamicClients(), true)
			ctx = authz.ContextWithActor(ctx, &user.User{Email: "user@example.com"}, nil)
			writes := 0
			client.PrependReactor("update", "postgres", func(action k8stesting.Action) (bool, runtime.Object, error) {
				writes++
				obj := action.(k8stesting.UpdateAction).GetObject()
				want := existing.DeepCopy()
				mustSetPostgresField(t, want.Object, "250m", "spec", "resources", "cpu")
				// The API adds deployment annotations, but must keep existing ones.
				want.SetAnnotations(obj.(metav1.Object).GetAnnotations())
				if diff := cmp.Diff(want, obj); diff != "" {
					t.Errorf("write lost original fields or resource version (-want +got):\n%s", diff)
				}
				return true, nil, k8serrors.NewConflict(gvr.GroupResource(), existing.GetName(), nil)
			})
			input := tt.input
			input.Name, input.TeamSlug, input.EnvironmentName = "orders", "team", "dev"
			payload, err := Update(ctx, input)
			if tt.wantWrite {
				if !k8serrors.IsConflict(err) || payload != nil || writes != 1 {
					t.Fatalf("Update = (%v, %v), writes %d; want one conflict without retry", payload, err, writes)
				}
			} else if err != nil || payload == nil || writes != 0 {
				t.Fatalf("no-op Update = (%v, %v), writes %d; want success without writes", payload, err, writes)
			}
			stored, err := resources.Get(t.Context(), existing.GetName(), metav1.GetOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(existing, stored); diff != "" {
				t.Errorf("unsuccessful/no-op write changed storage (-want +got):\n%s", diff)
			}
		})
	}
}
