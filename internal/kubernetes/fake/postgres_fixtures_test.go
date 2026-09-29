package fake_test

import (
	"os"
	"testing"

	"github.com/nais/api/internal/kubernetes"
	"github.com/nais/api/internal/kubernetes/fake"
)

func TestPostgresFixturesUseRegisteredV1Kinds(t *testing.T) {
	scheme, err := kubernetes.NewScheme()
	if err != nil {
		t.Fatal(err)
	}
	// Each directory is a separate integration suite; fixtures in different
	// suites may intentionally describe the same resource.
	for _, path := range []string{
		"../../../integration_tests/k8s_resources/create_postgres_access",
		"../../../integration_tests/k8s_resources/postgres_instances",
		"../../../integration_tests/k8s_resources/postgres_workloads",
		"../../../integration_tests/k8s_resources/postgres_delete",
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
