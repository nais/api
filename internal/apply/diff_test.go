package apply

import (
	"fmt"
	"testing"

	"github.com/nais/api/internal/activitylog"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestDiff_BothNil(t *testing.T) {
	changes := Diff(nil, nil)
	if len(changes) != 0 {
		t.Fatalf("expected no changes, got %d", len(changes))
	}
}

func TestDiff_BeforeNilCreatesAdded(t *testing.T) {
	after := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "nais.io/v1alpha1",
			"kind":       "Application",
			"metadata": map[string]any{
				"name":      "my-app",
				"namespace": "my-team",
			},
			"spec": map[string]any{
				"image":    "navikt/my-app:latest",
				"replicas": int64(2),
			},
		},
	}

	changes := Diff(nil, after)
	if len(changes) == 0 {
		t.Fatal("expected changes when before is nil")
	}

	fieldMap := toFieldMap(changes)
	assertFieldExists(t, fieldMap, "apiVersion")
	assertFieldExists(t, fieldMap, "kind")
	assertFieldExists(t, fieldMap, "metadata.name")
	assertFieldExists(t, fieldMap, "metadata.namespace")
	assertFieldExists(t, fieldMap, "spec.image")
	assertFieldExists(t, fieldMap, "spec.replicas")

	for _, c := range changes {
		if c.OldValue != nil {
			t.Errorf("expected OldValue to be nil for field %q when before is nil, got %v", c.Field, c.OldValue)
		}
	}
}

func TestDiff_AfterNilCreatesRemoved(t *testing.T) {
	before := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "nais.io/v1alpha1",
			"kind":       "Application",
			"spec": map[string]any{
				"image": "navikt/my-app:latest",
			},
		},
	}

	changes := Diff(before, nil)
	if len(changes) == 0 {
		t.Fatal("expected changes when after is nil")
	}

	for _, c := range changes {
		if c.NewValue != nil {
			t.Errorf("expected NewValue to be nil for field %q when after is nil, got %v", c.Field, c.NewValue)
		}
	}
}

func TestDiff_IdenticalObjects(t *testing.T) {
	obj := map[string]any{
		"apiVersion": "nais.io/v1alpha1",
		"kind":       "Application",
		"metadata": map[string]any{
			"name":      "my-app",
			"namespace": "my-team",
		},
		"spec": map[string]any{
			"image":    "navikt/my-app:latest",
			"replicas": int64(1),
		},
	}

	before := &unstructured.Unstructured{Object: deepCopyMap(obj)}
	after := &unstructured.Unstructured{Object: deepCopyMap(obj)}

	changes := Diff(before, after)
	if len(changes) != 0 {
		t.Fatalf("expected no changes for identical objects, got %d: %+v", len(changes), changes)
	}
}

func TestDiff_ScalarFieldChanged(t *testing.T) {
	before := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "nais.io/v1alpha1",
			"kind":       "Application",
			"spec": map[string]any{
				"image":    "navikt/my-app:v1",
				"replicas": int64(1),
			},
		},
	}

	after := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "nais.io/v1alpha1",
			"kind":       "Application",
			"spec": map[string]any{
				"image":    "navikt/my-app:v2",
				"replicas": int64(3),
			},
		},
	}

	changes := Diff(before, after)
	fieldMap := toFieldMap(changes)

	assertFieldChange(t, fieldMap, "spec.image", "navikt/my-app:v1", "navikt/my-app:v2")
	assertFieldChange(t, fieldMap, "spec.replicas", int64(1), int64(3))

	// apiVersion and kind are unchanged
	if _, ok := fieldMap["apiVersion"]; ok {
		t.Error("apiVersion should not appear in changes since it did not change")
	}
	if _, ok := fieldMap["kind"]; ok {
		t.Error("kind should not appear in changes since it did not change")
	}
}

func TestDiff_FieldAdded(t *testing.T) {
	before := &unstructured.Unstructured{
		Object: map[string]any{
			"spec": map[string]any{
				"image": "navikt/my-app:v1",
			},
		},
	}

	after := &unstructured.Unstructured{
		Object: map[string]any{
			"spec": map[string]any{
				"image":    "navikt/my-app:v1",
				"replicas": int64(2),
			},
		},
	}

	changes := Diff(before, after)
	fieldMap := toFieldMap(changes)

	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d: %+v", len(changes), changes)
	}

	c := fieldMap["spec.replicas"]
	if c.OldValue != nil {
		t.Errorf("expected OldValue to be nil, got %v", c.OldValue)
	}
	if c.NewValue == nil || *c.NewValue != "2" {
		t.Errorf("expected NewValue to be %q, got %v", "2", c.NewValue)
	}
}

func TestDiff_FieldRemoved(t *testing.T) {
	before := &unstructured.Unstructured{
		Object: map[string]any{
			"spec": map[string]any{
				"image":    "navikt/my-app:v1",
				"replicas": int64(2),
			},
		},
	}

	after := &unstructured.Unstructured{
		Object: map[string]any{
			"spec": map[string]any{
				"image": "navikt/my-app:v1",
			},
		},
	}

	changes := Diff(before, after)
	fieldMap := toFieldMap(changes)

	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d: %+v", len(changes), changes)
	}

	c := fieldMap["spec.replicas"]
	if c.OldValue == nil || *c.OldValue != "2" {
		t.Errorf("expected OldValue to be %q, got %v", "2", c.OldValue)
	}
	if c.NewValue != nil {
		t.Errorf("expected NewValue to be nil, got %v", c.NewValue)
	}
}

func TestDiff_NestedMapChanged(t *testing.T) {
	before := &unstructured.Unstructured{
		Object: map[string]any{
			"spec": map[string]any{
				"resources": map[string]any{
					"limits": map[string]any{
						"cpu":    "500m",
						"memory": "128Mi",
					},
				},
			},
		},
	}

	after := &unstructured.Unstructured{
		Object: map[string]any{
			"spec": map[string]any{
				"resources": map[string]any{
					"limits": map[string]any{
						"cpu":    "1000m",
						"memory": "256Mi",
					},
				},
			},
		},
	}

	changes := Diff(before, after)
	fieldMap := toFieldMap(changes)

	if len(changes) != 2 {
		t.Fatalf("expected 2 changes, got %d: %+v", len(changes), changes)
	}

	assertFieldChange(t, fieldMap, "spec.resources.limits.cpu", "500m", "1000m")
	assertFieldChange(t, fieldMap, "spec.resources.limits.memory", "128Mi", "256Mi")
}

func TestDiff_NestedMapAdded(t *testing.T) {
	before := &unstructured.Unstructured{
		Object: map[string]any{
			"spec": map[string]any{
				"image": "navikt/my-app:v1",
			},
		},
	}

	after := &unstructured.Unstructured{
		Object: map[string]any{
			"spec": map[string]any{
				"image": "navikt/my-app:v1",
				"resources": map[string]any{
					"limits": map[string]any{
						"cpu": "500m",
					},
				},
			},
		},
	}

	changes := Diff(before, after)
	fieldMap := toFieldMap(changes)

	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d: %+v", len(changes), changes)
	}

	assertFieldExists(t, fieldMap, "spec.resources.limits.cpu")
}

func TestDiff_SliceChanged(t *testing.T) {
	before := &unstructured.Unstructured{
		Object: map[string]any{
			"spec": map[string]any{
				"ingresses": []any{
					"https://old.example.com",
				},
			},
		},
	}

	after := &unstructured.Unstructured{
		Object: map[string]any{
			"spec": map[string]any{
				"ingresses": []any{
					"https://new.example.com",
				},
			},
		},
	}

	changes := Diff(before, after)
	fieldMap := toFieldMap(changes)

	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d: %+v", len(changes), changes)
	}

	assertFieldChange(t, fieldMap, "spec.ingresses[0]", "https://old.example.com", "https://new.example.com")
}

func TestDiff_SliceElementAdded(t *testing.T) {
	before := &unstructured.Unstructured{
		Object: map[string]any{
			"spec": map[string]any{
				"ingresses": []any{
					"https://one.example.com",
				},
			},
		},
	}

	after := &unstructured.Unstructured{
		Object: map[string]any{
			"spec": map[string]any{
				"ingresses": []any{
					"https://one.example.com",
					"https://two.example.com",
				},
			},
		},
	}

	changes := Diff(before, after)
	fieldMap := toFieldMap(changes)

	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d: %+v", len(changes), changes)
	}

	c := fieldMap["spec.ingresses[1]"]
	if c.OldValue != nil {
		t.Errorf("expected OldValue to be nil, got %v", c.OldValue)
	}
	if c.NewValue == nil || *c.NewValue != "https://two.example.com" {
		t.Errorf("expected NewValue to be %q, got %v", "https://two.example.com", c.NewValue)
	}
}

func TestDiff_SliceElementRemoved(t *testing.T) {
	before := &unstructured.Unstructured{
		Object: map[string]any{
			"spec": map[string]any{
				"ingresses": []any{
					"https://one.example.com",
					"https://two.example.com",
				},
			},
		},
	}

	after := &unstructured.Unstructured{
		Object: map[string]any{
			"spec": map[string]any{
				"ingresses": []any{
					"https://one.example.com",
				},
			},
		},
	}

	changes := Diff(before, after)
	fieldMap := toFieldMap(changes)

	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d: %+v", len(changes), changes)
	}

	c := fieldMap["spec.ingresses[1]"]
	if c.OldValue == nil || *c.OldValue != "https://two.example.com" {
		t.Errorf("expected OldValue to be %q, got %v", "https://two.example.com", c.OldValue)
	}
	if c.NewValue != nil {
		t.Errorf("expected NewValue to be nil, got %v", c.NewValue)
	}
}

func TestDiff_SliceOfMapsChanged(t *testing.T) {
	before := &unstructured.Unstructured{
		Object: map[string]any{
			"spec": map[string]any{
				"env": []any{
					map[string]any{"name": "FOO", "value": "bar"},
				},
			},
		},
	}

	after := &unstructured.Unstructured{
		Object: map[string]any{
			"spec": map[string]any{
				"env": []any{
					map[string]any{"name": "FOO", "value": "baz"},
				},
			},
		},
	}

	changes := Diff(before, after)
	fieldMap := toFieldMap(changes)

	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d: %+v", len(changes), changes)
	}

	assertFieldChange(t, fieldMap, "spec.env[0].value", "bar", "baz")
}

func TestDiff_IgnoresStatusField(t *testing.T) {
	before := &unstructured.Unstructured{
		Object: map[string]any{
			"spec": map[string]any{
				"image": "navikt/my-app:v1",
			},
			"status": map[string]any{
				"phase": "Running",
			},
		},
	}

	after := &unstructured.Unstructured{
		Object: map[string]any{
			"spec": map[string]any{
				"image": "navikt/my-app:v1",
			},
			"status": map[string]any{
				"phase": "Failed",
			},
		},
	}

	changes := Diff(before, after)
	if len(changes) != 0 {
		t.Fatalf("expected no changes (status should be ignored), got %d: %+v", len(changes), changes)
	}
}

func TestDiff_IgnoresServerManagedMetadataFields(t *testing.T) {
	before := &unstructured.Unstructured{
		Object: map[string]any{
			"metadata": map[string]any{
				"name":              "my-app",
				"namespace":         "my-team",
				"resourceVersion":   "12345",
				"uid":               "abc-def",
				"generation":        int64(1),
				"creationTimestamp": "2024-01-01T00:00:00Z",
				"managedFields":     []any{map[string]any{"manager": "kubectl"}},
				"selfLink":          "/api/v1/namespaces/my-team/my-app",
			},
		},
	}

	after := &unstructured.Unstructured{
		Object: map[string]any{
			"metadata": map[string]any{
				"name":              "my-app",
				"namespace":         "my-team",
				"resourceVersion":   "67890",
				"uid":               "xyz-123",
				"generation":        int64(2),
				"creationTimestamp": "2024-01-02T00:00:00Z",
				"managedFields":     []any{map[string]any{"manager": "nais-api"}},
				"selfLink":          "/api/v1/namespaces/my-team/my-app-2",
			},
		},
	}

	changes := Diff(before, after)
	if len(changes) != 0 {
		t.Fatalf("expected no changes (server-managed metadata should be ignored), got %d: %+v", len(changes), changes)
	}
}

func TestDiff_PreservesUserMetadataFields(t *testing.T) {
	before := &unstructured.Unstructured{
		Object: map[string]any{
			"metadata": map[string]any{
				"name":      "my-app",
				"namespace": "my-team",
				"labels": map[string]any{
					"app": "my-app",
				},
			},
		},
	}

	after := &unstructured.Unstructured{
		Object: map[string]any{
			"metadata": map[string]any{
				"name":      "my-app",
				"namespace": "my-team",
				"labels": map[string]any{
					"app":     "my-app",
					"version": "v2",
				},
			},
		},
	}

	changes := Diff(before, after)
	fieldMap := toFieldMap(changes)

	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d: %+v", len(changes), changes)
	}

	assertFieldExists(t, fieldMap, "metadata.labels.version")
}

func TestDiff_AnnotationsChanged(t *testing.T) {
	before := &unstructured.Unstructured{
		Object: map[string]any{
			"metadata": map[string]any{
				"name": "my-app",
				"annotations": map[string]any{
					"nais.io/cluster": "dev",
				},
			},
		},
	}

	after := &unstructured.Unstructured{
		Object: map[string]any{
			"metadata": map[string]any{
				"name": "my-app",
				"annotations": map[string]any{
					"nais.io/cluster":   "dev",
					"nais.io/something": "new",
				},
			},
		},
	}

	changes := Diff(before, after)
	fieldMap := toFieldMap(changes)

	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d: %+v", len(changes), changes)
	}

	assertFieldExists(t, fieldMap, "metadata.annotations.nais.io/something")
}

func TestDiff_TypeChangedScalar(t *testing.T) {
	before := &unstructured.Unstructured{
		Object: map[string]any{
			"spec": map[string]any{
				"value": "a-string",
			},
		},
	}

	after := &unstructured.Unstructured{
		Object: map[string]any{
			"spec": map[string]any{
				"value": int64(42),
			},
		},
	}

	changes := Diff(before, after)
	fieldMap := toFieldMap(changes)

	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d: %+v", len(changes), changes)
	}

	assertFieldChange(t, fieldMap, "spec.value", "a-string", int64(42))
}

func TestDiff_EmptyMapToPopulatedMap(t *testing.T) {
	before := &unstructured.Unstructured{
		Object: map[string]any{
			"spec": map[string]any{
				"labels": map[string]any{},
			},
		},
	}

	after := &unstructured.Unstructured{
		Object: map[string]any{
			"spec": map[string]any{
				"labels": map[string]any{
					"app": "my-app",
				},
			},
		},
	}

	changes := Diff(before, after)
	fieldMap := toFieldMap(changes)

	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d: %+v", len(changes), changes)
	}

	assertFieldExists(t, fieldMap, "spec.labels.app")
}

func TestDiff_BooleanChange(t *testing.T) {
	before := &unstructured.Unstructured{
		Object: map[string]any{
			"spec": map[string]any{
				"skipCaBundle": false,
			},
		},
	}

	after := &unstructured.Unstructured{
		Object: map[string]any{
			"spec": map[string]any{
				"skipCaBundle": true,
			},
		},
	}

	changes := Diff(before, after)
	fieldMap := toFieldMap(changes)

	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d: %+v", len(changes), changes)
	}

	assertFieldChange(t, fieldMap, "spec.skipCaBundle", false, true)
}

func TestDiff_MultipleTopLevelChanges(t *testing.T) {
	before := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "nais.io/v1alpha1",
			"kind":       "Application",
			"metadata": map[string]any{
				"name":      "my-app",
				"namespace": "my-team",
			},
			"spec": map[string]any{
				"image": "navikt/my-app:v1",
			},
		},
	}

	after := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "nais.io/v1alpha1",
			"kind":       "Application",
			"metadata": map[string]any{
				"name":      "my-app",
				"namespace": "other-team",
			},
			"spec": map[string]any{
				"image": "navikt/my-app:v2",
			},
		},
	}

	changes := Diff(before, after)
	fieldMap := toFieldMap(changes)

	if len(changes) != 2 {
		t.Fatalf("expected 2 changes, got %d: %+v", len(changes), changes)
	}

	assertFieldChange(t, fieldMap, "metadata.namespace", "my-team", "other-team")
	assertFieldChange(t, fieldMap, "spec.image", "navikt/my-app:v1", "navikt/my-app:v2")
}

func TestDiff_ResultsAreSorted(t *testing.T) {
	before := &unstructured.Unstructured{
		Object: map[string]any{
			"spec": map[string]any{
				"z_field": "a",
				"a_field": "a",
				"m_field": "a",
			},
		},
	}

	after := &unstructured.Unstructured{
		Object: map[string]any{
			"spec": map[string]any{
				"z_field": "b",
				"a_field": "b",
				"m_field": "b",
			},
		},
	}

	changes := Diff(before, after)
	if len(changes) != 3 {
		t.Fatalf("expected 3 changes, got %d", len(changes))
	}

	if changes[0].Field != "spec.a_field" {
		t.Errorf("expected first change to be spec.a_field, got %q", changes[0].Field)
	}
	if changes[1].Field != "spec.m_field" {
		t.Errorf("expected second change to be spec.m_field, got %q", changes[1].Field)
	}
	if changes[2].Field != "spec.z_field" {
		t.Errorf("expected third change to be spec.z_field, got %q", changes[2].Field)
	}
}

func TestDiff_DeletionTimestampIgnored(t *testing.T) {
	before := &unstructured.Unstructured{
		Object: map[string]any{
			"metadata": map[string]any{
				"name": "my-app",
			},
		},
	}

	after := &unstructured.Unstructured{
		Object: map[string]any{
			"metadata": map[string]any{
				"name":                       "my-app",
				"deletionTimestamp":          "2024-01-01T00:00:00Z",
				"deletionGracePeriodSeconds": int64(30),
			},
		},
	}

	changes := Diff(before, after)
	if len(changes) != 0 {
		t.Fatalf("expected no changes (deletion fields should be ignored), got %d: %+v", len(changes), changes)
	}
}

func TestDiff_EmptySliceToPopulatedSlice(t *testing.T) {
	before := &unstructured.Unstructured{
		Object: map[string]any{
			"spec": map[string]any{
				"ingresses": []any{},
			},
		},
	}

	after := &unstructured.Unstructured{
		Object: map[string]any{
			"spec": map[string]any{
				"ingresses": []any{
					"https://my-app.example.com",
				},
			},
		},
	}

	changes := Diff(before, after)
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d: %+v", len(changes), changes)
	}

	if changes[0].Field != "spec.ingresses[0]" {
		t.Errorf("expected field spec.ingresses[0], got %q", changes[0].Field)
	}
}

// --- Test helpers ---

func toFieldMap(changes []activitylog.ResourceChangedField) map[string]activitylog.ResourceChangedField {
	m := make(map[string]activitylog.ResourceChangedField, len(changes))
	for _, c := range changes {
		m[c.Field] = c
	}
	return m
}

func assertFieldExists(t *testing.T, fieldMap map[string]activitylog.ResourceChangedField, field string) {
	t.Helper()
	if _, ok := fieldMap[field]; !ok {
		t.Errorf("expected field %q to be present in changes, but it was not. Fields present: %v", field, fieldMapKeys(fieldMap))
	}
}

func assertFieldChange(t *testing.T, fieldMap map[string]activitylog.ResourceChangedField, field string, expectedOld, expectedNew any) {
	t.Helper()
	c, ok := fieldMap[field]
	if !ok {
		t.Errorf("expected field %q to be present in changes, but it was not. Fields present: %v", field, fieldMapKeys(fieldMap))
		return
	}

	wantOld := fmt.Sprintf("%v", expectedOld)
	wantNew := fmt.Sprintf("%v", expectedNew)

	if c.OldValue == nil || *c.OldValue != wantOld {
		got := "<nil>"
		if c.OldValue != nil {
			got = *c.OldValue
		}
		t.Errorf("field %q: expected OldValue %q, got %q", field, wantOld, got)
	}
	if c.NewValue == nil || *c.NewValue != wantNew {
		got := "<nil>"
		if c.NewValue != nil {
			got = *c.NewValue
		}
		t.Errorf("field %q: expected NewValue %q, got %q", field, wantNew, got)
	}
}

func fieldMapKeys(m map[string]activitylog.ResourceChangedField) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func deepCopyMap(m map[string]any) map[string]any {
	cp := make(map[string]any, len(m))
	for k, v := range m {
		switch val := v.(type) {
		case map[string]any:
			cp[k] = deepCopyMap(val)
		case []any:
			cp[k] = deepCopySlice(val)
		default:
			cp[k] = v
		}
	}
	return cp
}

func deepCopySlice(s []any) []any {
	cp := make([]any, len(s))
	for i, v := range s {
		switch val := v.(type) {
		case map[string]any:
			cp[i] = deepCopyMap(val)
		case []any:
			cp[i] = deepCopySlice(val)
		default:
			cp[i] = v
		}
	}
	return cp
}
