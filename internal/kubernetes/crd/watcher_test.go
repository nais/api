package crd

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/nais/api/internal/kubernetes"
	"github.com/nais/api/internal/kubernetes/fake"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/sirupsen/logrus/hooks/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
)

var majorVersions = Enum{
	Name: "postgresMajorVersions", CRDName: "postgres.nais.io",
	Version: "v1", Path: []string{"spec", "majorVersion"},
}

func init() {
	RegisterEnum(majorVersions)
}

func TestRegisterEnumRejectsInvalidDefinitions(t *testing.T) {
	for _, tt := range []struct {
		name   string
		change func(*Enum)
		want   string
	}{
		{name: "missing name", change: func(e *Enum) { e.Name = "" }, want: "requires"},
		{name: "missing definition", change: func(e *Enum) { e.CRDName = "" }, want: "requires"},
		{name: "missing version", change: func(e *Enum) { e.Version = "" }, want: "requires"},
		{name: "missing path", change: func(e *Enum) { e.Path = nil }, want: "requires"},
		{name: "empty property", change: func(e *Enum) { e.Path = []string{"spec", ""} }, want: "requires"},
		{name: "duplicate name", want: "duplicate"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			field := majorVersions
			if tt.change != nil {
				tt.change(&field)
			}
			defer func() {
				value := recover()
				message, ok := value.(string)
				if !ok || !strings.Contains(message, tt.want) {
					t.Errorf("RegisterEnum() panic = %v, want containing %q", value, tt.want)
				}
			}()
			RegisterEnum(field)
		})
	}
}

func definition(values ...any) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apiextensions.k8s.io/v1",
		"kind":       "CustomResourceDefinition",
		"metadata":   map[string]any{"name": "postgres.nais.io"},
		"spec": map[string]any{
			"group": "nais.io",
			"names": map[string]any{"kind": "Postgres", "plural": "postgres"},
			"scope": "Namespaced",
			"versions": []any{map[string]any{
				"name": "v1", "served": true, "storage": true,
				"schema": map[string]any{"openAPIV3Schema": map[string]any{
					"type": "object",
					"properties": map[string]any{"spec": map[string]any{
						"type": "object",
						"properties": map[string]any{"majorVersion": map[string]any{
							"type": "string", "enum": values,
						}},
					}},
				}},
			}},
		},
	}}
}

func definitionWithProperty(t *testing.T, property map[string]any) *unstructured.Unstructured {
	t.Helper()
	obj := definition()
	version := obj.Object["spec"].(map[string]any)["versions"].([]any)[0].(map[string]any)
	if err := unstructured.SetNestedMap(version, property, "schema", "openAPIV3Schema", "properties", "spec", "properties", "majorVersion"); err != nil {
		t.Fatal(err)
	}
	return obj
}

func TestExtractEnum(t *testing.T) {
	for _, tt := range []struct {
		name   string
		change func(*unstructured.Unstructured)
		want   []string
		err    string
	}{
		{name: "preserves source ordering", want: []string{"18", "17", "16"}},
		{name: "wrong version", change: func(o *unstructured.Unstructured) {
			o.Object["spec"].(map[string]any)["versions"].([]any)[0].(map[string]any)["name"] = "v2"
		}, err: "schema version"},
		{name: "unserved version", change: func(o *unstructured.Unstructured) {
			o.Object["spec"].(map[string]any)["versions"].([]any)[0].(map[string]any)["served"] = false
		}, err: "not served"},
		{name: "missing schema", change: func(o *unstructured.Unstructured) {
			delete(o.Object["spec"].(map[string]any)["versions"].([]any)[0].(map[string]any), "schema")
		}, err: "no string enum"},
		{name: "empty enum", change: func(o *unstructured.Unstructured) {
			*o = *definition()
		}, err: "no string enum"},
		{name: "non-string enum", change: func(o *unstructured.Unstructured) {
			*o = *definition(int64(18))
		}, err: "string"},
		{name: "malformed versions", change: func(o *unstructured.Unstructured) {
			o.Object["spec"].(map[string]any)["versions"] = "invalid"
		}, err: "must be a list"},
		{name: "selects configured version", change: func(o *unstructured.Unstructured) {
			spec := o.Object["spec"].(map[string]any)
			spec["versions"] = append([]any{map[string]any{"name": "v2", "served": true}}, spec["versions"].([]any)...)
		}, want: []string{"18", "17", "16"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			obj := definition("18", "17", "16")
			if tt.change != nil {
				tt.change(obj)
			}
			got, err := majorVersions.extract(obj)
			if tt.err != "" {
				if err == nil || !strings.Contains(err.Error(), tt.err) {
					t.Fatalf("extract() error = %v, want containing %q", err, tt.err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(&StringOptions{Values: tt.want}, got); diff != "" {
				t.Errorf("values mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestExtractStringDefaults(t *testing.T) {
	for _, tt := range []struct {
		name     string
		property map[string]any
		want     *StringOptions
		err      string
	}{
		{
			name:     "scalar without default",
			property: map[string]any{"type": "string", "enum": []any{"a", "b"}},
			want:     &StringOptions{Values: []string{"a", "b"}},
		},
		{
			name:     "scalar default",
			property: map[string]any{"type": "string", "enum": []any{"a", "b"}, "default": "b"},
			want:     &StringOptions{Values: []string{"a", "b"}, DefaultValues: []string{"b"}},
		},
		{
			name:     "empty string is a default",
			property: map[string]any{"type": "string", "enum": []any{"", "a"}, "default": ""},
			want:     &StringOptions{Values: []string{"", "a"}, DefaultValues: []string{""}},
		},
		{
			name:     "scalar rejects multiple defaults",
			property: map[string]any{"type": "string", "enum": []any{"a", "b"}, "default": []any{"a", "b"}},
			err:      "expected string",
		},
		{
			name:     "scalar rejects numeric default",
			property: map[string]any{"type": "string", "enum": []any{"a", "b"}, "default": int64(1)},
			err:      "expected string",
		},
		{
			name:     "scalar default must be allowed",
			property: map[string]any{"type": "string", "enum": []any{"a", "b"}, "default": "c"},
			err:      "not in the enum",
		},
		{
			name:     "array without default",
			property: map[string]any{"type": "array", "items": map[string]any{"type": "string", "enum": []any{"a", "b"}}},
			want:     &StringOptions{Values: []string{"a", "b"}},
		},
		{
			name:     "item default is not an array selection",
			property: map[string]any{"type": "array", "items": map[string]any{"type": "string", "enum": []any{"a", "b"}, "default": "a"}},
			want:     &StringOptions{Values: []string{"a", "b"}},
		},
		{
			name:     "array with empty default",
			property: map[string]any{"type": "array", "items": map[string]any{"type": "string", "enum": []any{"a", "b"}}, "default": []any{}},
			want:     &StringOptions{Values: []string{"a", "b"}, DefaultValues: []string{}},
		},
		{
			name:     "array with single default",
			property: map[string]any{"type": "array", "items": map[string]any{"type": "string", "enum": []any{"a", "b"}}, "default": []any{"b"}},
			want:     &StringOptions{Values: []string{"a", "b"}, DefaultValues: []string{"b"}},
		},
		{
			name:     "array with multiple defaults preserves ordering",
			property: map[string]any{"type": "array", "items": map[string]any{"type": "string", "enum": []any{"a", "b"}}, "default": []any{"b", "a"}},
			want:     &StringOptions{Values: []string{"a", "b"}, DefaultValues: []string{"b", "a"}},
		},
		{
			name:     "array rejects scalar default",
			property: map[string]any{"type": "array", "items": map[string]any{"type": "string", "enum": []any{"a", "b"}}, "default": "a"},
			err:      "expected []interface{}",
		},
		{
			name:     "array rejects non-string default",
			property: map[string]any{"type": "array", "items": map[string]any{"type": "string", "enum": []any{"a", "b"}}, "default": []any{"a", int64(1)}},
			err:      "non-string",
		},
		{
			name:     "array defaults must be allowed",
			property: map[string]any{"type": "array", "items": map[string]any{"type": "string", "enum": []any{"a", "b"}}, "default": []any{"a", "c"}},
			err:      "not in the enum",
		},
		{
			name:     "array requires string items",
			property: map[string]any{"type": "array", "items": map[string]any{"type": "integer", "enum": []any{int64(1)}}},
			err:      "items must be strings",
		},
		{
			name:     "array requires an item enum",
			property: map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			err:      "no string enum",
		},
		{
			name:     "non-string settings are unsupported",
			property: map[string]any{"type": "integer", "enum": []any{int64(1)}},
			err:      "expected a string",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := majorVersions.extract(definitionWithProperty(t, tt.property))
			if tt.err != "" {
				if err == nil || !strings.Contains(err.Error(), tt.err) {
					t.Fatalf("extract() error = %v, want containing %q", err, tt.err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("options mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCatalogWatchesChanges(t *testing.T) {
	scheme, err := kubernetes.NewScheme()
	if err != nil {
		t.Fatal(err)
	}
	log, _ := test.NewNullLogger()
	mgr, err := watcher.NewManager(scheme, kubernetes.ClusterConfigMap{"dev": nil, "staging": nil},
		log, watcher.WithClientCreator(fake.Clients(nil)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mgr.Stop)
	catalog := NewWatcher(t.Context(), mgr)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	if !mgr.WaitForReady(ctx) {
		t.Fatal("catalog did not synchronize")
	}
	waitForOptions := func(environment string, want *StringOptions) {
		t.Helper()
		var got *StringOptions
		var getErr error
		err := wait.PollUntilContextTimeout(t.Context(), 10*time.Millisecond, 5*time.Second, true, func(context.Context) (bool, error) {
			got, getErr = catalog.Options(environment, majorVersions.Name)
			if want == nil {
				return getErr != nil, nil
			}
			return getErr == nil && cmp.Equal(want, got), nil
		})
		if err != nil {
			t.Fatalf("%s values = %v, error = %v; want %v: %v", environment, got, getErr, want, err)
		}
	}
	waitForOptions("dev", nil)
	if _, err := catalog.Options("missing", majorVersions.Name); err == nil {
		t.Error("unknown environment must not have values")
	}
	if _, err := catalog.Options("dev", "unknown"); err == nil {
		t.Error("unregistered enum must not have values")
	}
	gvr := schema.GroupVersionResource{Group: "apiextensions.k8s.io", Version: "v1", Resource: "customresourcedefinitions"}
	client := mgr.GetDynamicClients()["dev"].Resource(gvr)
	obj := definition("18", "17")
	obj.SetAnnotations(map[string]string{"kubectl.kubernetes.io/last-applied-configuration": strings.Repeat("x", 100_000)})
	if _, err := client.Create(t.Context(), obj, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	waitForOptions("dev", &StringOptions{Values: []string{"18", "17"}})
	if _, err := mgr.GetDynamicClients()["staging"].Resource(gvr).Create(t.Context(), definition("16"), metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	waitForOptions("staging", &StringOptions{Values: []string{"16"}})
	cached, err := catalog.watcher.Get("dev", "", majorVersions.CRDName)
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := cached.Object["spec"]; exists || len(cached.GetAnnotations()) != 0 {
		t.Error("catalog retained full schema or annotations")
	}
	projected, err := catalog.transform(cached.DeepCopy())
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(cached, projected); diff != "" {
		t.Errorf("transform must be idempotent (-want +got):\n%s", diff)
	}
	options, err := catalog.Options("dev", majorVersions.Name)
	if err != nil {
		t.Fatal(err)
	}
	options.Values[0] = "corrupted"
	waitForOptions("dev", &StringOptions{Values: []string{"18", "17"}})
	for _, defaultValue := range []string{"17", "18"} {
		updated := definitionWithProperty(t, map[string]any{"type": "string", "enum": []any{"18", "17"}, "default": defaultValue})
		if _, err := client.Update(t.Context(), updated, metav1.UpdateOptions{}); err != nil {
			t.Fatal(err)
		}
		waitForOptions("dev", &StringOptions{Values: []string{"18", "17"}, DefaultValues: []string{defaultValue}})
	}
	options, err = catalog.Options("dev", majorVersions.Name)
	if err != nil {
		t.Fatal(err)
	}
	options.DefaultValues[0] = "corrupted"
	waitForOptions("dev", &StringOptions{Values: []string{"18", "17"}, DefaultValues: []string{"18"}})
	invalidDefault := definitionWithProperty(t, map[string]any{"type": "string", "enum": []any{"18", "17"}, "default": "19"})
	if _, err := client.Update(t.Context(), invalidDefault, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	waitForOptions("dev", nil)
	if _, err := client.Update(t.Context(), definition("19", "18"), metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	waitForOptions("dev", &StringOptions{Values: []string{"19", "18"}})
	waitForOptions("staging", &StringOptions{Values: []string{"16"}})
	for _, defaults := range [][]string{{"18", "19"}, {}, nil} {
		property := map[string]any{"type": "array", "items": map[string]any{"type": "string", "enum": []any{"19", "18"}}}
		if defaults != nil {
			if err := unstructured.SetNestedStringSlice(property, defaults, "default"); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := client.Update(t.Context(), definitionWithProperty(t, property), metav1.UpdateOptions{}); err != nil {
			t.Fatal(err)
		}
		waitForOptions("dev", &StringOptions{Values: []string{"19", "18"}, DefaultValues: defaults})
	}
	if _, err := client.Update(t.Context(), definition(int64(19)), metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	waitForOptions("dev", nil)
	if _, err := client.Update(t.Context(), definition("20"), metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	waitForOptions("dev", &StringOptions{Values: []string{"20"}})
	if err := client.Delete(t.Context(), obj.GetName(), metav1.DeleteOptions{}); err != nil {
		t.Fatal(err)
	}
	waitForOptions("dev", nil)
	if _, err := client.Create(t.Context(), definition("21"), metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	waitForOptions("dev", &StringOptions{Values: []string{"21"}})
}

func TestCatalogUnavailableWithoutContext(t *testing.T) {
	if _, err := Options(t.Context(), "dev", majorVersions.Name); err == nil {
		t.Fatal("expected an unavailable catalog error")
	} else if public, ok := err.(interface{ GraphError() string }); !ok || !strings.Contains(public.GraphError(), "unavailable") {
		t.Fatalf("expected a public availability error, got %v", err)
	}
}
