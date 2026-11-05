package postgres

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nais/api/internal/graph/model"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestPostgresUpdatePreservesUnrequestedFields(t *testing.T) {
	for _, tt := range []struct {
		name   string
		input  UpdatePostgresInput
		change func(map[string]any)
	}{
		{name: "no changes"},
		{
			name: "semantically unchanged request",
			input: UpdatePostgresInput{
				HighAvailability: new(true), Memory: new("1Gi"), Extensions: []string{"pg_trgm"},
				Labels: []*model.ResourceLabel{{Key: "tier", Value: "backend"}},
			},
		},
		{
			name:  "CPU only",
			input: UpdatePostgresInput{CPU: new("250m")},
			change: func(object map[string]any) {
				mustSetPostgresField(t, object, "250m", "spec", "resources", "cpu")
			},
		},
		{
			name:  "memory only",
			input: UpdatePostgresInput{Memory: new("2Gi")},
			change: func(object map[string]any) {
				mustSetPostgresField(t, object, "2Gi", "spec", "resources", "memory")
			},
		},
		{
			name:  "disk only",
			input: UpdatePostgresInput{DiskSize: new("20Gi")},
			change: func(object map[string]any) {
				mustSetPostgresField(t, object, "20Gi", "spec", "resources", "diskSize")
			},
		},
		{
			name:  "disable high availability",
			input: UpdatePostgresInput{HighAvailability: new(false)},
			change: func(object map[string]any) {
				unstructured.RemoveNestedField(object, "spec", "highAvailability")
			},
		},
		{
			name:  "replace extensions",
			input: UpdatePostgresInput{Extensions: []string{"citext"}},
			change: func(object map[string]any) {
				mustSetPostgresField(t, object, []any{map[string]any{"name": "citext"}}, "spec", "extensions")
			},
		},
		{
			name:  "remove extensions",
			input: UpdatePostgresInput{Extensions: []string{}},
			change: func(object map[string]any) {
				unstructured.RemoveNestedField(object, "spec", "extensions")
			},
		},
		{
			name:  "replace user labels",
			input: UpdatePostgresInput{Labels: []*model.ResourceLabel{{Key: "owner", Value: "ledger"}}},
			change: func(object map[string]any) {
				unstructured.RemoveNestedField(object, "metadata", "labels", "tier")
				mustSetPostgresField(t, object, "ledger", "metadata", "labels", "owner")
			},
		},
		{
			name:  "remove user labels",
			input: UpdatePostgresInput{Labels: []*model.ResourceLabel{}},
			change: func(object map[string]any) {
				unstructured.RemoveNestedField(object, "metadata", "labels", "tier")
			},
		},
		{
			name:  "equivalent quantity stays byte-for-byte unchanged during another update",
			input: UpdatePostgresInput{Memory: new("1Gi"), CPU: new("250m")},
			change: func(object map[string]any) {
				mustSetPostgresField(t, object, "250m", "spec", "resources", "cpu")
			},
		},
		{
			name:  "unchanged extension names preserve newer extension fields",
			input: UpdatePostgresInput{Extensions: []string{"pg_trgm"}, CPU: new("250m")},
			change: func(object map[string]any) {
				mustSetPostgresField(t, object, "250m", "spec", "resources", "cpu")
			},
		},
		{
			name: "combined update",
			input: UpdatePostgresInput{
				HighAvailability: new(false), CPU: new("250m"), Memory: new("2Gi"),
				DiskSize: new("20Gi"), Extensions: []string{}, Labels: []*model.ResourceLabel{},
			},
			change: func(object map[string]any) {
				unstructured.RemoveNestedField(object, "metadata", "labels", "tier")
				unstructured.RemoveNestedField(object, "spec", "highAvailability")
				unstructured.RemoveNestedField(object, "spec", "extensions")
				mustSetPostgresField(t, object, "250m", "spec", "resources", "cpu")
				mustSetPostgresField(t, object, "2Gi", "spec", "resources", "memory")
				mustSetPostgresField(t, object, "20Gi", "spec", "resources", "diskSize")
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			existing := postgresWithNewerFields()
			original := existing.DeepCopy()
			want := existing.DeepCopy()
			if tt.change != nil {
				tt.change(want.Object)
			}
			got, changes, err := preparePostgresUpdate(existing, tt.input)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(want.Object, got.Object); diff != "" {
				t.Errorf("update changed unrequested fields (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(original.Object, existing.Object); diff != "" {
				t.Errorf("preparation mutated its source (-want +got):\n%s", diff)
			}
			if (len(changes) != 0) != (tt.change != nil) {
				t.Errorf("audit changes = %v, want changed = %t", changes, tt.change != nil)
			}
		})
	}
}

func TestPostgresUpdateDoesNotAddOmittedResources(t *testing.T) {
	existing := postgresWithNewerFields()
	unstructured.RemoveNestedField(existing.Object, "spec", "resources")
	want := existing.DeepCopy()
	mustSetPostgresField(t, want.Object, "250m", "spec", "resources", "cpu")
	got, _, err := preparePostgresUpdate(existing, UpdatePostgresInput{CPU: new("250m")})
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(want.Object, got.Object); diff != "" {
		t.Errorf("omitted resource fields must remain absent (-want +got):\n%s", diff)
	}
}

func TestPostgresUpdatePreservesSourceOnInvalidQuantity(t *testing.T) {
	existing := postgresWithNewerFields()
	original := existing.DeepCopy()
	got, changes, err := preparePostgresUpdate(existing, UpdatePostgresInput{
		HighAvailability: new(false), CPU: new("not-a-quantity"),
	})
	if err == nil || got != nil || len(changes) != 0 {
		t.Fatalf("prepare = (%v, %v, %v), want no update on invalid input", got, changes, err)
	}
	if diff := cmp.Diff(original.Object, existing.Object); diff != "" {
		t.Errorf("invalid update mutated source (-want +got):\n%s", diff)
	}
}

func postgresWithNewerFields() *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "nais.io/v1", "kind": "Postgres",
		"metadata": map[string]any{
			"name": "orders", "namespace": "team", "uid": "postgres-uid", "resourceVersion": "73",
			"annotations":     map[string]any{"postgres.nais.io/physical-identity": "historical-resources"},
			"labels":          map[string]any{"app.kubernetes.io/managed-by": "console", "postgres.nais.io/name": "orders", "tier": "backend"},
			"finalizers":      []any{"postgres.nais.io"},
			"ownerReferences": []any{map[string]any{"apiVersion": "nais.io/v1", "kind": "Example", "name": "owner", "uid": "owner-uid"}},
		},
		"spec": map[string]any{
			"majorVersion": "18", "highAvailability": true,
			"activeBranch":           "main",
			"resources":              map[string]any{"cpu": "100m", "memory": "1024Mi", "diskSize": "10Gi", "futureIOPS": int64(3000)},
			"extensions":             []any{map[string]any{"name": "pg_trgm", "futureSchema": "public"}},
			"futurePhysicalIdentity": map[string]any{"cluster": "pg-existing-main", "archive": "existing-archive"},
			"futureBootstrap":        map[string]any{"recovery": map[string]any{"source": "source-archive"}},
		},
		"status": map[string]any{"activeBranch": "main", "futureMigrationPhase": "prepared"},
	}}
}

func mustSetPostgresField(t *testing.T, object map[string]any, value any, fields ...string) {
	t.Helper()
	if err := unstructured.SetNestedField(object, value, fields...); err != nil {
		t.Fatal(err)
	}
}
