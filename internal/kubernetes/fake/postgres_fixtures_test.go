package fake_test

import (
	"context"
	"os"
	"testing"

	"github.com/nais/api/internal/kubernetes"
	"github.com/nais/api/internal/kubernetes/fake"
	nais_io_v1 "github.com/nais/pgrator/pkg/api/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestPostgresBranchKindResolver(t *testing.T) {
	_, kinds, _, err := fake.Clients(nil)("dev")
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := kinds.KindsFor(schema.GroupVersionResource{Group: "nais.io", Version: "v1", Resource: "postgresbranches"})
	if err != nil || len(resolved) != 1 || resolved[0].Kind != "PostgresBranch" {
		guessed, _ := meta.UnsafeGuessKindToResource(schema.GroupVersion{Group: "nais.io", Version: "v1"}.WithKind("PostgresBranch"))
		t.Fatalf("resolving PostgresBranch resource: %v, %v (guessed %s)", resolved, err, guessed.Resource)
	}
}

func TestPostgresFixturesUseRegisteredV1Kinds(t *testing.T) {
	scheme, err := kubernetes.NewScheme()
	if err != nil {
		t.Fatal(err)
	}
	// Each directory is a separate integration suite; fixtures in different
	// suites may intentionally describe the same resource.
	for _, path := range []string{
		"../../../integration_tests/k8s_resources/create_postgres_access",
		"../../../integration_tests/k8s_resources/postgres_branches",
		"../../../integration_tests/k8s_resources/postgres_workloads",
		"../../../integration_tests/k8s_resources/postgres_audit_log",
		"../../../integration_tests/k8s_resources/label_selectors",
		"../../../data/k8s",
	} {
		resources, err := fake.ParseResources(scheme, os.DirFS(path))
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if len(resources) == 0 {
			t.Errorf("%s: no fixtures", path)
		}
		// Parsing alone does not detect two files declaring the same resource.
		// One fake tracker per cluster and suite catches duplicate identities
		// across all files belonging to that cluster, as in the Lua runner.
		for _, objects := range resources {
			client := fake.NewDynamicClient(scheme)
			fake.AddObjectToDynamicClient(scheme, client, objects...)
		}
	}
}

func TestLocalPostgresFixturesPublishDirectClusterIdentity(t *testing.T) {
	client, _, _, err := fake.Clients(os.DirFS("../../../data/k8s"))("dev")
	if err != nil {
		t.Fatal(err)
	}

	postgreses := client.Resource(nais_io_v1.GroupVersion.WithResource("postgres")).Namespace("devteam")
	list, err := postgreses.List(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Items) < 2 {
		t.Fatalf("expected multiple local Postgres fixtures, got %d", len(list.Items))
	}

	for _, postgres := range list.Items {
		name := postgres.GetName()
		cluster, found, err := unstructured.NestedString(postgres.Object, "status", "clusterName")
		if err != nil {
			t.Fatal(err)
		}
		if !found || cluster == "" {
			t.Errorf("Postgres %s has no observed cluster", name)
			continue
		}
		if want := nais_io_v1.CNPGClusterName(nais_io_v1.PostgresPhysicalName(name)); cluster != want {
			t.Errorf("cluster identity = %q, want %q", cluster, want)
		}
	}
}
