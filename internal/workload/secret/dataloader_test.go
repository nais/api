package secret

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// TestTransformSecretIdempotent verifies that transformSecret is idempotent.
// The Kubernetes informer's WatchList code path can run the transformer twice
// on the same object: once in a temporary store, and again in DeltaFIFO.Replace().
// If transformSecret is not idempotent, the second run would see no "data" field
// (already removed by the first run) and overwrite the cached-secret-keys
// annotation with an empty string, causing keys to disappear.
func TestTransformSecretIdempotent(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]any{
				"name":      "my-secret",
				"namespace": "my-team",
				"labels": map[string]any{
					"nais.io/managed-by": "console",
				},
				"annotations": map[string]any{
					"console.nais.io/last-modified-at": "2024-10-18T12:44:57Z",
					"console.nais.io/last-modified-by": "user@example.com",
				},
			},
			"type": "Opaque",
			"data": map[string]any{
				"DATABASE_URL": "cG9zdGdyZXM6Ly9sb2NhbGhvc3QvbXlkYg==",
				"API_KEY":      "c2VjcmV0LWtleQ==",
			},
		},
	}

	// First transform: should extract keys and remove data
	result1, err := transformSecret(obj)
	if err != nil {
		t.Fatalf("first transformSecret call failed: %v", err)
	}

	secret1 := result1.(*unstructured.Unstructured)
	keys1 := secret1.GetAnnotations()[annotationSecretKeys]
	if keys1 != "API_KEY,DATABASE_URL" {
		t.Fatalf("after first transform: expected keys annotation %q, got %q", "API_KEY,DATABASE_URL", keys1)
	}

	// data should be removed
	if _, exists, _ := unstructured.NestedMap(secret1.Object, "data"); exists {
		t.Fatal("after first transform: data field should have been removed")
	}

	// Second transform (simulates WatchList double-transform): keys must be preserved
	result2, err := transformSecret(secret1)
	if err != nil {
		t.Fatalf("second transformSecret call failed: %v", err)
	}

	secret2 := result2.(*unstructured.Unstructured)
	keys2 := secret2.GetAnnotations()[annotationSecretKeys]
	if keys2 != keys1 {
		t.Fatalf("after second transform: keys annotation changed from %q to %q (transformer is not idempotent)", keys1, keys2)
	}

	// Verify the full round-trip through toGraphSecret still works
	graphSecret, ok := toGraphSecret(secret2, "dev")
	if !ok {
		t.Fatal("toGraphSecret returned false after double transform")
	}

	if len(graphSecret.Keys) != 2 {
		t.Fatalf("expected 2 keys, got %d: %v", len(graphSecret.Keys), graphSecret.Keys)
	}
	if graphSecret.Keys[0] != "API_KEY" || graphSecret.Keys[1] != "DATABASE_URL" {
		t.Fatalf("expected keys [API_KEY, DATABASE_URL], got %v", graphSecret.Keys)
	}
}

// TestTransformSecretEmptyData verifies that a secret with no data keys
// is handled correctly through double-transform.
func TestTransformSecretEmptyDataIdempotent(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]any{
				"name":      "empty-secret",
				"namespace": "my-team",
				"labels": map[string]any{
					"nais.io/managed-by": "console",
				},
			},
			"type": "Opaque",
			"data": map[string]any{},
		},
	}

	result1, err := transformSecret(obj)
	if err != nil {
		t.Fatalf("first transform failed: %v", err)
	}

	secret1 := result1.(*unstructured.Unstructured)
	keys1 := secret1.GetAnnotations()[annotationSecretKeys]
	if keys1 != "" {
		t.Fatalf("expected empty keys annotation for empty secret, got %q", keys1)
	}

	// Second transform should not break anything
	result2, err := transformSecret(secret1)
	if err != nil {
		t.Fatalf("second transform failed: %v", err)
	}

	secret2 := result2.(*unstructured.Unstructured)
	keys2 := secret2.GetAnnotations()[annotationSecretKeys]
	if keys2 != "" {
		t.Fatalf("expected empty keys annotation after second transform, got %q", keys2)
	}

	graphSecret, ok := toGraphSecret(secret2, "dev")
	if !ok {
		t.Fatal("toGraphSecret returned false")
	}
	if len(graphSecret.Keys) != 0 {
		t.Fatalf("expected 0 keys, got %d: %v", len(graphSecret.Keys), graphSecret.Keys)
	}
}

// TestTransformSecretNoDataField verifies that a secret without any data
// field at all (e.g. freshly created) is handled correctly.
func TestTransformSecretNoDataFieldIdempotent(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]any{
				"name":      "new-secret",
				"namespace": "my-team",
				"labels": map[string]any{
					"nais.io/managed-by": "console",
				},
			},
			"type": "Opaque",
		},
	}

	result1, err := transformSecret(obj)
	if err != nil {
		t.Fatalf("first transform failed: %v", err)
	}

	secret1 := result1.(*unstructured.Unstructured)

	result2, err := transformSecret(secret1)
	if err != nil {
		t.Fatalf("second transform failed: %v", err)
	}

	secret2 := result2.(*unstructured.Unstructured)
	graphSecret, ok := toGraphSecret(secret2, "dev")
	if !ok {
		t.Fatal("toGraphSecret returned false")
	}
	if len(graphSecret.Keys) != 0 {
		t.Fatalf("expected 0 keys, got %d: %v", len(graphSecret.Keys), graphSecret.Keys)
	}
}
