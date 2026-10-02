package apply

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nais/api/internal/activitylog"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/managedfields"
	dynfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestApplyResource_RemoveCPULimit(t *testing.T) {
	for _, test := range []struct {
		name               string
		previousManager    string
		applyBeforeRemoval bool
		changeCPU          bool
	}{
		{
			name: "removes a field owned only by nais-api",
		},
		{
			name:            "removes a field owned by another manager",
			previousManager: "previous-deployer",
		},
		{
			name:               "removes a field shared with another manager",
			previousManager:    "previous-deployer",
			applyBeforeRemoval: true,
		},
		{
			name:               "removes a field after forcefully taking ownership",
			previousManager:    "previous-deployer",
			applyBeforeRemoval: true,
			changeCPU:          true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx := t.Context()
			gvr := schema.GroupVersionResource{Group: "nais.io", Version: "v1alpha1", Resource: "applications"}
			client := newFieldManagedDynamicClient(gvr.GroupVersion().WithKind("Application"))
			resourceClient := client.Resource(gvr).Namespace("my-team")
			application := &unstructured.Unstructured{Object: map[string]any{
				"apiVersion": "nais.io/v1alpha1",
				"kind":       "Application",
				"metadata": map[string]any{
					"name":      "my-app",
					"namespace": "my-team",
				},
				"spec": map[string]any{
					"image": "example.com/my-app:v1",
					"resources": map[string]any{
						"limits": map[string]any{
							"cpu":    "1",
							"memory": "512Mi",
						},
						"requests": map[string]any{
							"cpu":    "100m",
							"memory": "256Mi",
						},
					},
				},
			}}

			if test.previousManager == "" {
				result, err := ApplyResource(ctx, client, gvr, application)
				if err != nil {
					t.Fatal(err)
				}
				if !result.Created || result.Before != nil {
					t.Fatal("expected application to be created without a before-state")
				}
				if diff := cmp.Diff(application.Object["spec"], result.After.Object["spec"]); diff != "" {
					t.Fatalf("created spec mismatch (-want +got):\n%s", diff)
				}
			} else {
				data, err := json.Marshal(application.Object)
				if err != nil {
					t.Fatal(err)
				}
				_, err = resourceClient.Patch(ctx, application.GetName(), types.ApplyPatchType, data, metav1.PatchOptions{
					FieldManager: test.previousManager,
				})
				if err != nil {
					t.Fatal(err)
				}
			}

			oldCPU := "1"
			if test.applyBeforeRemoval {
				if test.changeCPU {
					oldCPU = "2"
					if err := unstructured.SetNestedField(application.Object, oldCPU, "spec", "resources", "limits", "cpu"); err != nil {
						t.Fatal(err)
					}
				}
				data, err := json.Marshal(application.Object)
				if err != nil {
					t.Fatal(err)
				}
				applied, err := resourceClient.Patch(ctx, application.GetName(), types.ApplyPatchType, data, metav1.PatchOptions{
					FieldManager: fieldManager,
					Force:        new(true),
				})
				if err != nil {
					t.Fatal(err)
				}
				if diff := cmp.Diff(application.Object["spec"], applied.Object["spec"]); diff != "" {
					t.Fatalf("applied spec mismatch (-want +got):\n%s", diff)
				}
			}

			desired := application.DeepCopy()
			unstructured.RemoveNestedField(desired.Object, "spec", "resources", "limits", "cpu")

			result, err := ApplyResource(ctx, client, gvr, desired)
			if err != nil {
				t.Fatal(err)
			}
			if result.Created || result.Before == nil {
				t.Fatal("expected existing application to be applied with a before-state")
			}
			beforeCPU, found, err := unstructured.NestedString(result.Before.Object, "spec", "resources", "limits", "cpu")
			if err != nil {
				t.Fatal(err)
			}
			if !found || beforeCPU != oldCPU {
				t.Fatalf("CPU limit before apply = %q (found: %t), want %q", beforeCPU, found, oldCPU)
			}
			afterCPU, found, err := unstructured.NestedString(result.After.Object, "spec", "resources", "limits", "cpu")
			if err != nil {
				t.Fatal(err)
			}
			if found {
				t.Fatalf("spec.resources.limits.cpu is still present after apply: %q", afterCPU)
			}
			if diff := cmp.Diff(desired.Object["spec"], result.After.Object["spec"]); diff != "" {
				t.Errorf("applied spec mismatch (-want +got):\n%s", diff)
			}

			changes := Diff(result.Before, result.After)
			wantChanges := []activitylog.ResourceChangedField{{
				Field:    "spec.resources.limits.cpu",
				OldValue: &oldCPU,
			}}
			if diff := cmp.Diff(wantChanges, changes); diff != "" {
				t.Errorf("changed fields mismatch (-want +got):\n%s", diff)
			}

			stored, err := resourceClient.Get(ctx, application.GetName(), metav1.GetOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(result.After.Object, stored.Object); diff != "" {
				t.Errorf("stored application mismatch (-want +got):\n%s", diff)
			}

			reapplied, err := ApplyResource(ctx, client, gvr, desired)
			if err != nil {
				t.Fatal(err)
			}
			if reapplied.Created {
				t.Error("expected existing application to be reapplied, not created")
			}
			if diff := cmp.Diff(desired.Object["spec"], reapplied.After.Object["spec"]); diff != "" {
				t.Errorf("reapplied spec mismatch (-want +got):\n%s", diff)
			}
			if changes := Diff(reapplied.Before, reapplied.After); len(changes) != 0 {
				t.Errorf("expected no changes when reapplying, got %+v", changes)
			}
		})
	}
}

func TestApplyResource_ReplacesEntireObject(t *testing.T) {
	for _, test := range []struct {
		kind          string
		gvr           schema.GroupVersionResource
		beforeFields  map[string]any
		desiredFields map[string]any
	}{
		{
			kind: "Application",
			gvr:  schema.GroupVersionResource{Group: "nais.io", Version: "v1alpha1", Resource: "applications"},
			beforeFields: map[string]any{
				"spec": map[string]any{
					"image": "example.com/my-app:v1",
					"resources": map[string]any{
						"limits": map[string]any{"cpu": "1"},
					},
				},
			},
			desiredFields: map[string]any{
				"spec": map[string]any{"image": "example.com/my-app:v2"},
			},
		},
		{
			kind: "ConfigMap",
			gvr:  schema.GroupVersionResource{Version: "v1", Resource: "configmaps"},
			beforeFields: map[string]any{
				"data":       map[string]any{"kept": "old", "removed": "old"},
				"binaryData": map[string]any{"removed": "b2xk"},
			},
			desiredFields: map[string]any{
				"data": map[string]any{"kept": "new"},
			},
		},
		{
			kind: "Secret",
			gvr:  schema.GroupVersionResource{Version: "v1", Resource: "secrets"},
			beforeFields: map[string]any{
				"data": map[string]any{"kept": "b2xk", "removed": "b2xk"},
				"type": "Opaque",
			},
			desiredFields: map[string]any{
				"data": map[string]any{"kept": "bmV3"},
			},
		},
	} {
		t.Run(test.kind, func(t *testing.T) {
			ctx := t.Context()
			gvk := test.gvr.GroupVersion().WithKind(test.kind)
			client := newFieldManagedDynamicClient(gvk)
			resourceClient := client.Resource(test.gvr).Namespace("my-team")
			existing := (&unstructured.Unstructured{Object: test.beforeFields}).DeepCopy()
			existing.SetGroupVersionKind(gvk)
			existing.SetName("my-resource")
			existing.SetNamespace("my-team")
			existing.SetLabels(map[string]string{"kept": "same", "removed": "old"})
			existing.SetAnnotations(map[string]string{"removed": "old"})

			data, err := json.Marshal(existing.Object)
			if err != nil {
				t.Fatal(err)
			}
			_, err = resourceClient.Patch(ctx, existing.GetName(), types.ApplyPatchType, data, metav1.PatchOptions{
				FieldManager: "previous-deployer",
			})
			if err != nil {
				t.Fatal(err)
			}

			desired := (&unstructured.Unstructured{Object: test.desiredFields}).DeepCopy()
			desired.SetGroupVersionKind(gvk)
			desired.SetName(existing.GetName())
			desired.SetNamespace(existing.GetNamespace())
			desired.SetLabels(map[string]string{"kept": "same", "added": "new"})
			input := desired.DeepCopy()
			client.ClearActions()

			result, err := ApplyResource(ctx, client, test.gvr, desired)
			if err != nil {
				t.Fatal(err)
			}
			if result.Created || result.Before == nil {
				t.Fatal("expected replacement of an existing resource")
			}
			if changes := Diff(existing, result.Before); len(changes) != 0 {
				t.Fatalf("before-state differs from existing resource: %+v", changes)
			}

			actual := result.After.DeepCopy()
			actual.SetManagedFields(nil)
			actual.SetResourceVersion("")
			if diff := cmp.Diff(input.Object, actual.Object); diff != "" {
				t.Errorf("replacement mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(input.Object, desired.Object); diff != "" {
				t.Errorf("input was mutated (-want +got):\n%s", diff)
			}

			actions := client.Actions()
			if len(actions) != 2 || actions[0].GetVerb() != "get" || actions[1].GetVerb() != "update" {
				t.Fatalf("expected GET followed by UPDATE, got %v", actions)
			}
			update, ok := actions[1].(k8stesting.UpdateActionImpl)
			if !ok {
				t.Fatalf("expected UpdateActionImpl, got %T", actions[1])
			}
			options := update.GetUpdateOptions()
			if options.FieldManager != fieldManager || options.FieldValidation != metav1.FieldValidationStrict {
				t.Errorf("unexpected update options: %+v", options)
			}
			stored, err := resourceClient.Get(ctx, desired.GetName(), metav1.GetOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(result.After.Object, stored.Object); diff != "" {
				t.Errorf("stored replacement mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestApplyResource_UsesCurrentResourceVersion(t *testing.T) {
	gvr := schema.GroupVersionResource{Group: "nais.io", Version: "v1alpha1", Resource: "applications"}
	client := dynfake.NewSimpleDynamicClient(runtime.NewScheme())
	existing := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "nais.io/v1alpha1",
		"kind":       "Application",
		"metadata": map[string]any{
			"name":            "my-app",
			"namespace":       "my-team",
			"resourceVersion": "42",
		},
		"spec": map[string]any{"image": "example.com/my-app:v1"},
	}}
	desired := existing.DeepCopy()
	desired.SetResourceVersion("")
	if err := unstructured.SetNestedField(desired.Object, "example.com/my-app:v2", "spec", "image"); err != nil {
		t.Fatal(err)
	}
	input := desired.DeepCopy()

	client.PrependReactor("get", "*", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, existing.DeepCopy(), nil
	})
	client.PrependReactor("update", "*", func(action k8stesting.Action) (bool, runtime.Object, error) {
		update, ok := action.(k8stesting.UpdateAction)
		if !ok {
			t.Fatalf("expected UpdateAction, got %T", action)
		}
		object, ok := update.GetObject().(*unstructured.Unstructured)
		if !ok {
			t.Fatalf("expected unstructured object, got %T", update.GetObject())
		}
		expected := desired.DeepCopy()
		expected.SetResourceVersion("42")
		if diff := cmp.Diff(expected.Object, object.Object); diff != "" {
			t.Errorf("update mismatch (-want +got):\n%s", diff)
		}
		return true, object.DeepCopy(), nil
	})

	result, err := ApplyResource(t.Context(), client, gvr, desired)
	if err != nil {
		t.Fatal(err)
	}
	if result.Created {
		t.Error("expected an update, not a creation")
	}
	if diff := cmp.Diff(existing.Object, result.Before.Object); diff != "" {
		t.Errorf("before-state mismatch (-want +got):\n%s", diff)
	}
	if result.After.GetResourceVersion() != "42" {
		t.Errorf("resourceVersion = %q, want 42", result.After.GetResourceVersion())
	}
	if diff := cmp.Diff(input.Object, desired.Object); diff != "" {
		t.Errorf("input was mutated (-want +got):\n%s", diff)
	}
}

func TestApplyResource_KubernetesErrors(t *testing.T) {
	gvr := schema.GroupVersionResource{Group: "nais.io", Version: "v1alpha1", Resource: "applications"}
	for _, test := range []struct {
		name string
		verb string
		err  error
	}{
		{
			name: "get forbidden",
			verb: "get",
			err:  apierrors.NewForbidden(gvr.GroupResource(), "my-app", errors.New("denied")),
		},
		{
			name: "create rejected",
			verb: "create",
			err:  apierrors.NewBadRequest("invalid resource"),
		},
		{
			name: "created concurrently",
			verb: "create",
			err:  apierrors.NewAlreadyExists(gvr.GroupResource(), "my-app"),
		},
		{
			name: "update rejected",
			verb: "update",
			err:  apierrors.NewBadRequest("immutable field changed"),
		},
		{
			name: "updated concurrently",
			verb: "update",
			err:  apierrors.NewConflict(gvr.GroupResource(), "my-app", errors.New("resourceVersion changed")),
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := dynfake.NewSimpleDynamicClient(runtime.NewScheme())
			desired := &unstructured.Unstructured{Object: map[string]any{
				"apiVersion": "nais.io/v1alpha1",
				"kind":       "Application",
				"metadata": map[string]any{
					"name":      "my-app",
					"namespace": "my-team",
				},
			}}
			if test.verb == "update" {
				existing := desired.DeepCopy()
				existing.SetResourceVersion("42")
				client.PrependReactor("get", "*", func(k8stesting.Action) (bool, runtime.Object, error) {
					return true, existing.DeepCopy(), nil
				})
			}
			client.PrependReactor(test.verb, "*", func(k8stesting.Action) (bool, runtime.Object, error) {
				return true, nil, test.err
			})

			result, err := ApplyResource(t.Context(), client, gvr, desired)
			if !errors.Is(err, test.err) {
				t.Fatalf("error = %v, want wrapped %v", err, test.err)
			}
			if result != nil {
				t.Errorf("expected no result on failure, got %+v", result)
			}
			wantActions := 2
			if test.verb == "get" {
				wantActions = 1
			}
			if actions := client.Actions(); len(actions) != wantActions || actions[len(actions)-1].GetVerb() != test.verb {
				t.Errorf("unexpected actions after failure: %v", actions)
			}
		})
	}
}

func newFieldManagedDynamicClient(gvk schema.GroupVersionKind) *dynfake.FakeDynamicClient {
	scheme := runtime.NewScheme()
	scheme.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(gvk.GroupVersion().WithKind(gvk.Kind+"List"), &unstructured.UnstructuredList{})

	// The integration fake replaces spec wholesale, masking field ownership.
	// Use Kubernetes' field manager to include existing field ownership.
	tracker := k8stesting.NewFieldManagedObjectTracker(scheme, serializer.NewCodecFactory(scheme).UniversalDecoder(), managedfields.NewDeducedTypeConverter())
	client := dynfake.NewSimpleDynamicClient(scheme)
	client.PrependReactor("*", "*", k8stesting.ObjectReaction(tracker))
	return client
}
