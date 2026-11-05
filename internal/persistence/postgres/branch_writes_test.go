package postgres

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/kubernetes"
	"github.com/nais/api/internal/kubernetes/fake"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/sirupsen/logrus/hooks/test"
)

func TestDisabledBranchWritesRejectBeforeAccessingKubernetes(t *testing.T) {
	// No watchers or clients: a disabled operation must return before looking up
	// resources, writing to Kubernetes or recording a successful audit event.
	ctx := NewLoaderContext(t.Context(), nil, nil, "", "", "nav", nil, true)
	for _, tt := range []struct {
		name string
		run  func(*testing.T, context.Context) error
	}{
		{"create", func(t *testing.T, ctx context.Context) error {
			payload, err := CreateBranch(ctx, CreatePostgresBranchInput{
				Postgres: "orders", Branch: "recovery", SourceBranch: "main",
				TargetTime: time.Now().Add(-time.Hour).Truncate(time.Second), TeamSlug: "team", EnvironmentName: "dev",
			})
			if payload != nil {
				t.Fatal("disabled create returned a success payload")
			}
			return err
		}},
		{"activate", func(t *testing.T, ctx context.Context) error {
			payload, err := ActivateBranch(ctx, ActivatePostgresBranchInput{Postgres: "orders", Branch: "main", TeamSlug: "team", EnvironmentName: "dev"})
			if payload != nil {
				t.Fatal("disabled activation returned a success payload")
			}
			return err
		}},
		{"delete", func(t *testing.T, ctx context.Context) error {
			payload, err := Delete(ctx, DeletePostgresBranchInput{Postgres: "orders", Branch: "recovery", TeamSlug: "team", EnvironmentName: "dev"})
			if payload != nil {
				t.Fatal("disabled deletion returned a success payload")
			}
			return err
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run(t, ctx)
			if !errors.Is(err, errBranchWritesDisabled) {
				t.Fatalf("error = %v, want the branch write freeze", err)
			}
			var publicError apierror.Error
			if !errors.As(err, &publicError) {
				t.Fatalf("error %T must be visible to GraphQL clients", err)
			}
		})
	}
}

func TestBranchWriteFreezePreservesReadsAndAccessValidation(t *testing.T) {
	scheme, err := kubernetes.NewScheme()
	if err != nil {
		t.Fatal(err)
	}
	clusters, err := kubernetes.CreateClusterConfigMap("nav", []string{"dev"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	log, _ := test.NewNullLogger()
	mgr, err := watcher.NewManager(scheme, clusters, log, watcher.WithClientCreator(fake.Clients(os.DirFS("../../../integration_tests/k8s_resources/create_postgres_access"))))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mgr.Stop)
	branchWatcher := NewPostgresBranchWatcher(t.Context(), mgr)
	postgresWatcher := NewPostgresWatcher(t.Context(), mgr)
	wait, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	if !mgr.WaitForReady(wait) {
		t.Fatal("watchers did not synchronize")
	}
	ctx := NewLoaderContext(t.Context(), branchWatcher, postgresWatcher, "", "", "nav", mgr.GetDynamicClients(), true)
	if _, err := GetPostgres(ctx, "someteamname", "dev", "foobar"); err != nil {
		t.Fatalf("database read is frozen: %v", err)
	}
	if _, err := GetReadyPostgresBranch(ctx, "someteamname", "dev", "foobar", "main"); err != nil {
		t.Fatalf("branch readiness read is frozen: %v", err)
	}
	input := CreatePostgresAccessInput{
		Postgres: "foobar", Branch: "main", TeamSlug: "someteamname", EnvironmentName: "dev",
		AccessLevel: PostgresAccessLevelRead, Reason: "Inspecting the database during the transition",
	}
	if err := input.Validate(ctx); err != nil {
		t.Fatalf("personal access validation is frozen: %v", err)
	}
}
