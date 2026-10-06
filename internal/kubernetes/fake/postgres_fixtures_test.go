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
		"../../../integration_tests/k8s_resources/postgres_branch_delete",
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

func TestLocalPostgresFixturesHaveResolvableActiveBranches(t *testing.T) {
	client, _, _, err := fake.Clients(os.DirFS("../../../data/k8s"))("dev")
	if err != nil {
		t.Fatal(err)
	}

	postgreses := client.Resource(nais_io_v1.GroupVersion.WithResource("postgres")).Namespace("devteam")
	branches := client.Resource(nais_io_v1.GroupVersion.WithResource("postgresbranches")).Namespace("devteam")
	list, err := postgreses.List(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Items) < 2 {
		t.Fatalf("expected multiple local Postgres fixtures, got %d", len(list.Items))
	}

	for _, postgres := range list.Items {
		name := postgres.GetName()
		branch, found, err := unstructured.NestedString(postgres.Object, "status", "activeBranch")
		if err != nil {
			t.Fatal(err)
		}
		if !found || branch == "" {
			t.Errorf("Postgres %s has no observed active branch", name)
			continue
		}
		requested, _, err := unstructured.NestedString(postgres.Object, "spec", "activeBranch")
		if err != nil {
			t.Fatal(err)
		}
		if requested != branch {
			t.Errorf("Postgres %s requests branch %q but observes %q", name, requested, branch)
		}
		objectName := nais_io_v1.PostgresBranchObjectName(name, branch)
		if _, err := branches.Get(context.Background(), objectName, metav1.GetOptions{}); err != nil {
			t.Errorf("active branch %s/%s must resolve to PostgresBranch %s: %v", name, branch, objectName, err)
		}
	}
}
