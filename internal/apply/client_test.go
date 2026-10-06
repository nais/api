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
	"k8s.io/client-go/dynamic"
	dynfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestApplyResource_RemoveCPULimit(t *testing.T) {
	for _, test := range []struct {
		name               string
		previousManager    string
		previousApply      bool
		applyBeforeRemoval bool
		controllerOwnsCPU  bool
		wantCPU            bool
	}{
		{name: "owned only by nais-api"},
		{name: "created using kubectl create", previousManager: "kubectl-create"},
		{name: "managed by client-side kubectl apply", previousManager: "kubectl-client-side-apply"},
		{name: "managed by deployd", previousManager: "deployd"},
		{name: "unchanged kubectl-create ownership is migrated", previousManager: "kubectl-create", applyBeforeRemoval: true},
		{name: "unchanged kubectl apply ownership is migrated", previousManager: "kubectl-client-side-apply", applyBeforeRemoval: true},
		{name: "unchanged deployd ownership is migrated", previousManager: "deployd", applyBeforeRemoval: true},
		{name: "controller-owned field is retained", previousManager: "cpu-controller", wantCPU: true},
		{name: "field shared with a controller is retained", previousManager: "cpu-controller", applyBeforeRemoval: true, wantCPU: true},
		{name: "migrated field shared with a controller is retained", previousManager: "deployd", controllerOwnsCPU: true, wantCPU: true},
		{name: "legacy Apply entries are not migrated", previousManager: "deployd", previousApply: true, wantCPU: true},
		{name: "server-side kubectl ownership is retained", previousManager: "kubectl", previousApply: true, wantCPU: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx := t.Context()
			gvr := schema.GroupVersionResource{Group: "nais.io", Version: "v1alpha1", Resource: "applications"}
			client := newFieldManagedDynamicClient(gvr.GroupVersion().WithKind("Application"))
			resourceClient := client.Resource(gvr).Namespace("my-team")
			application := applyTestApplication()
			if test.previousManager == "" {
				result, err := ApplyResource(ctx, client, gvr, application)
				if err != nil {
					t.Fatal(err)
				}
				if !result.Created || result.Before != nil {
					t.Fatal("expected creation without a before-state")
				}
			} else {
				snapshot := application.DeepCopy()
				// The tracker does not assign resourceVersions like a real API server.
				snapshot.SetResourceVersion("1")
				if test.previousApply {
					applyAsManager(t, resourceClient, snapshot, test.previousManager)
				} else if _, err := resourceClient.Create(ctx, snapshot, metav1.CreateOptions{
					FieldManager: test.previousManager,
				}); err != nil {
					t.Fatal(err)
				}
			}
			if test.controllerOwnsCPU {
				controllerIntent := application.DeepCopy()
				controllerIntent.Object["spec"] = map[string]any{
					"resources": map[string]any{"limits": map[string]any{"cpu": "1"}},
				}
				applyAsManager(t, resourceClient, controllerIntent, "cpu-controller")
			}
			if test.applyBeforeRemoval {
				result, err := ApplyResource(ctx, client, gvr, application)
				if err != nil {
					t.Fatal(err)
				}
				if result.Created || len(Diff(result.Before, result.After)) != 0 {
					t.Fatal("expected unchanged application to be applied without configuration changes")
				}
				if !test.wantCPU {
					for _, entry := range result.After.GetManagedFields() {
						if entry.Manager == test.previousManager {
							t.Fatalf("legacy ownership was not transferred: %+v", entry)
						}
					}
				}
			}
			desired := application.DeepCopy()
			unstructured.RemoveNestedField(desired.Object, "spec", "resources", "limits", "cpu")
			input := desired.DeepCopy()

			result, err := ApplyResource(ctx, client, gvr, desired)
			if err != nil {
				t.Fatal(err)
			}
			if result.Created || result.Before == nil {
				t.Fatal("expected an existing application with a before-state")
			}
			beforeCPU, found, err := unstructured.NestedString(result.Before.Object, "spec", "resources", "limits", "cpu")
			if err != nil || !found || beforeCPU != "1" {
				t.Fatalf("CPU before apply = %q (found=%t), err=%v", beforeCPU, found, err)
			}
			expected := desired
			if test.wantCPU {
				expected = application
			}
			if diff := cmp.Diff(expected.Object["spec"], result.After.Object["spec"]); diff != "" {
				t.Errorf("applied spec mismatch (-want +got):\n%s", diff)
			}
			changes := Diff(result.Before, result.After)
			if test.wantCPU {
				if len(changes) != 0 {
					t.Errorf("controller-owned field changed: %+v", changes)
				}
			} else {
				oldCPU := "1"
				wantChanges := []activitylog.ResourceChangedField{{Field: "spec.resources.limits.cpu", OldValue: &oldCPU}}
				if diff := cmp.Diff(wantChanges, changes); diff != "" {
					t.Errorf("changed fields mismatch (-want +got):\n%s", diff)
				}
			}
			if diff := cmp.Diff(input.Object, desired.Object); diff != "" {
				t.Errorf("input was mutated (-want +got):\n%s", diff)
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
			if reapplied.Created || len(Diff(reapplied.Before, reapplied.After)) != 0 {
				t.Error("expected reapplication without configuration changes")
			}
			if diff := cmp.Diff(expected.Object["spec"], reapplied.After.Object["spec"]); diff != "" {
				t.Errorf("reapplied spec mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestApplyResource_RemovesOmittedLegacyFields(t *testing.T) {
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
				"spec": map[string]any{"image": "example.com/my-app:v1", "resources": map[string]any{"limits": map[string]any{"cpu": "1"}}},
			},
			desiredFields: map[string]any{"spec": map[string]any{"image": "example.com/my-app:v2"}},
		},
		{
			kind: "ConfigMap",
			gvr:  schema.GroupVersionResource{Version: "v1", Resource: "configmaps"},
			beforeFields: map[string]any{
				"data":       map[string]any{"kept": "old", "removed": "old"},
				"binaryData": map[string]any{"removed": "b2xk"},
			},
			desiredFields: map[string]any{"data": map[string]any{"kept": "new"}},
		},
		{
			kind:          "Secret",
			gvr:           schema.GroupVersionResource{Version: "v1", Resource: "secrets"},
			beforeFields:  map[string]any{"data": map[string]any{"kept": "b2xk", "removed": "b2xk"}},
			desiredFields: map[string]any{"data": map[string]any{"kept": "bmV3"}},
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
			existing.SetResourceVersion("42")
			existing.SetLabels(map[string]string{"kept": "same", "removed": "old"})
			existing.SetAnnotations(map[string]string{"removed": "old"})
			if _, err := resourceClient.Create(ctx, existing, metav1.CreateOptions{FieldManager: "deployd"}); err != nil {
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
				t.Fatal("expected an existing resource")
			}
			if changes := Diff(existing, result.Before); len(changes) != 0 {
				t.Fatalf("before-state differs from existing resource: %+v", changes)
			}
			actual := result.After.DeepCopy()
			actual.SetManagedFields(nil)
			actual.SetResourceVersion("")
			if diff := cmp.Diff(input.Object, actual.Object); diff != "" {
				t.Errorf("applied object mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(input.Object, desired.Object); diff != "" {
				t.Errorf("input was mutated (-want +got):\n%s", diff)
			}
			actions := client.Actions()
			if len(actions) != 3 || actions[0].GetVerb() != "get" {
				t.Fatalf("expected GET, migration PATCH, and SSA PATCH, got %v", actions)
			}
			for i, patchType := range []types.PatchType{types.JSONPatchType, types.ApplyPatchType} {
				action, ok := actions[i+1].(k8stesting.PatchActionImpl)
				if !ok || action.GetPatchType() != patchType {
					t.Fatalf("action %d is not %s: %v", i+1, patchType, actions[i+1])
				}
				options := action.GetPatchOptions()
				if options.FieldManager != fieldManager {
					t.Errorf("unexpected field manager: %q", options.FieldManager)
				}
				if patchType == types.ApplyPatchType &&
					(options.Force == nil || *options.Force || options.FieldValidation != metav1.FieldValidationStrict) {
					t.Errorf("unexpected SSA options: %+v", options)
				}
			}
		})
	}
}

func TestApplyResource_PreservesControllerFields(t *testing.T) {
	gvr := schema.GroupVersionResource{Group: "nais.io", Version: "v1alpha1", Resource: "applications"}
	client := newFieldManagedDynamicClient(gvr.GroupVersion().WithKind("Application"))
	resourceClient := client.Resource(gvr).Namespace("my-team")
	application := applyTestApplication()
	snapshot := application.DeepCopy()
	snapshot.SetResourceVersion("1")
	if _, err := resourceClient.Create(t.Context(), snapshot, metav1.CreateOptions{FieldManager: "deployd"}); err != nil {
		t.Fatal(err)
	}
	controllerIntent := application.DeepCopy()
	delete(controllerIntent.Object, "spec")
	controllerIntent.SetAnnotations(map[string]string{"controller": "kept"})
	controllerIntent.SetLabels(map[string]string{"controller": "kept"})
	controllerIntent.SetFinalizers([]string{"controller.example.com/cleanup"})
	controllerIntent.SetOwnerReferences([]metav1.OwnerReference{{
		APIVersion: "nais.io/v1alpha1", Kind: "Application", Name: "owner", UID: "owner-uid",
		Controller: new(true), BlockOwnerDeletion: new(true),
	}})
	applyAsManager(t, resourceClient, controllerIntent, "resource-controller")
	desired := application.DeepCopy()
	unstructured.RemoveNestedField(desired.Object, "spec", "resources", "limits", "cpu")

	result, err := ApplyResource(t.Context(), client, gvr, desired)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(controllerIntent.Object["metadata"], metadataWithoutBookkeeping(result.After)); diff != "" {
		t.Errorf("controller metadata mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(desired.Object["spec"], result.After.Object["spec"]); diff != "" {
		t.Errorf("legacy field removal mismatch (-want +got):\n%s", diff)
	}
	for _, entry := range result.Before.GetManagedFields() {
		if entry.Manager == "resource-controller" {
			found := false
			for _, after := range result.After.GetManagedFields() {
				if after.Manager == entry.Manager {
					found = true
					if diff := cmp.Diff(entry, after); diff != "" {
						t.Errorf("controller ownership changed (-want +got):\n%s", diff)
					}
				}
			}
			if !found {
				t.Error("controller ownership was removed")
			}
		}
	}
}

func TestApplyResource_RejectsControllerConflicts(t *testing.T) {
	gvr := schema.GroupVersionResource{Group: "nais.io", Version: "v1alpha1", Resource: "applications"}
	client := newFieldManagedDynamicClient(gvr.GroupVersion().WithKind("Application"))
	resourceClient := client.Resource(gvr).Namespace("my-team")
	existing := applyTestApplication()
	before := applyAsManager(t, resourceClient, existing, "resource-controller")
	desired := existing.DeepCopy()
	if err := unstructured.SetNestedField(desired.Object, "2", "spec", "resources", "limits", "cpu"); err != nil {
		t.Fatal(err)
	}
	result, err := ApplyResource(t.Context(), client, gvr, desired)
	if !apierrors.IsConflict(err) || result != nil {
		t.Fatalf("expected an ownership conflict, got result=%+v, err=%v", result, err)
	}
	stored, err := resourceClient.Get(t.Context(), existing.GetName(), metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(before.Object, stored.Object); diff != "" {
		t.Errorf("conflicting apply modified the resource (-want +got):\n%s", diff)
	}
}

func TestApplyResource_PreservesServerAssignedFields(t *testing.T) {
	for _, test := range []struct {
		kind       string
		gvr        schema.GroupVersionResource
		manifest   map[string]any
		controller map[string]any
	}{
		{
			kind: "Service",
			gvr:  schema.GroupVersionResource{Version: "v1", Resource: "services"},
			manifest: map[string]any{"spec": map[string]any{
				"selector": map[string]any{"app": "my-app"},
				"ports":    []any{map[string]any{"port": int64(80)}},
			}},
			controller: map[string]any{"spec": map[string]any{
				"clusterIP": "10.0.0.42", "clusterIPs": []any{"10.0.0.42"},
			}},
		},
		{
			kind: "Job",
			gvr:  schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "jobs"},
			manifest: map[string]any{"spec": map[string]any{"template": map[string]any{
				"spec": map[string]any{
					"restartPolicy": "Never",
					"containers":    []any{map[string]any{"name": "app", "image": "example.com/app:v1"}},
				},
			}}},
			controller: map[string]any{"spec": map[string]any{
				"selector": map[string]any{"matchLabels": map[string]any{"batch.kubernetes.io/controller-uid": "job-uid"}},
				"template": map[string]any{"metadata": map[string]any{"labels": map[string]any{
					"batch.kubernetes.io/controller-uid": "job-uid", "batch.kubernetes.io/job-name": "my-resource",
				}}},
			}},
		},
	} {
		t.Run(test.kind, func(t *testing.T) {
			gvk := test.gvr.GroupVersion().WithKind(test.kind)
			client := newFieldManagedDynamicClient(gvk)
			resourceClient := client.Resource(test.gvr).Namespace("my-team")
			manifest := (&unstructured.Unstructured{Object: test.manifest}).DeepCopy()
			manifest.SetGroupVersionKind(gvk)
			manifest.SetName("my-resource")
			manifest.SetNamespace("my-team")
			snapshot := manifest.DeepCopy()
			snapshot.SetResourceVersion("1")
			if _, err := resourceClient.Create(t.Context(), snapshot, metav1.CreateOptions{FieldManager: "kubectl-create"}); err != nil {
				t.Fatal(err)
			}
			controller := (&unstructured.Unstructured{Object: test.controller}).DeepCopy()
			controller.SetGroupVersionKind(gvk)
			controller.SetName(manifest.GetName())
			controller.SetNamespace(manifest.GetNamespace())
			live := applyAsManager(t, resourceClient, controller, "kube-controller-manager")

			result, err := ApplyResource(t.Context(), client, test.gvr, manifest)
			if err != nil {
				t.Fatal(err)
			}
			if changes := Diff(result.Before, result.After); len(changes) != 0 {
				t.Errorf("unchanged manifest removed server-assigned fields: %+v", changes)
			}
			if diff := cmp.Diff(live.Object["spec"], result.After.Object["spec"]); diff != "" {
				t.Errorf("server-assigned fields changed (-want +got):\n%s", diff)
			}
		})
	}
}

func TestApplyResource_MigrationResourceVersionConflict(t *testing.T) {
	gvr := schema.GroupVersionResource{Group: "nais.io", Version: "v1alpha1", Resource: "applications"}
	client := newFieldManagedDynamicClient(gvr.GroupVersion().WithKind("Application"))
	resourceClient := client.Resource(gvr).Namespace("my-team")
	desired := applyTestApplication()
	snapshot := desired.DeepCopy()
	snapshot.SetResourceVersion("42")
	if _, err := resourceClient.Create(t.Context(), snapshot, metav1.CreateOptions{FieldManager: "deployd"}); err != nil {
		t.Fatal(err)
	}
	input := desired.DeepCopy()
	client.ClearActions()
	conflict := apierrors.NewConflict(gvr.GroupResource(), desired.GetName(), errors.New("resourceVersion changed"))
	client.PrependReactor("patch", "*", func(action k8stesting.Action) (bool, runtime.Object, error) {
		patch, ok := action.(k8stesting.PatchAction)
		if !ok || patch.GetPatchType() != types.JSONPatchType {
			t.Fatalf("expected migration PATCH before SSA, got %v", action)
		}
		var operations []struct {
			Op    string
			Path  string
			Value json.RawMessage
		}
		if err := json.Unmarshal(patch.GetPatch(), &operations); err != nil {
			t.Fatal(err)
		}
		guarded := false
		for _, operation := range operations {
			if operation.Op == "replace" && operation.Path == "/metadata/resourceVersion" {
				guarded = true
				if string(operation.Value) != `"42"` {
					t.Errorf("migration resourceVersion = %s, want 42", operation.Value)
				}
			}
		}
		if !guarded {
			t.Error("ownership migration has no resourceVersion guard")
		}
		return true, nil, conflict
	})
	result, err := ApplyResource(t.Context(), client, gvr, desired)
	if !errors.Is(err, conflict) || result != nil {
		t.Fatalf("expected wrapped migration conflict, got result=%+v, err=%v", result, err)
	}
	if actions := client.Actions(); len(actions) != 2 {
		t.Errorf("expected no SSA after migration failure, got %v", actions)
	}
	if diff := cmp.Diff(input.Object, desired.Object); diff != "" {
		t.Errorf("input was mutated (-want +got):\n%s", diff)
	}
}

func TestApplyResource_DoesNotMigrateStatusOwnership(t *testing.T) {
	gvr := schema.GroupVersionResource{Group: "nais.io", Version: "v1alpha1", Resource: "applications"}
	client := dynfake.NewSimpleDynamicClient(runtime.NewScheme())
	desired := applyTestApplication()
	before := desired.DeepCopy()
	before.SetResourceVersion("42")
	statusOwner := metav1.ManagedFieldsEntry{
		Manager: "deployd", Operation: metav1.ManagedFieldsOperationUpdate,
		APIVersion: desired.GetAPIVersion(), Subresource: "status", FieldsType: "FieldsV1",
		FieldsV1: &metav1.FieldsV1{Raw: []byte(`{"f:status":{"f:state":{}}}`)},
	}
	before.SetManagedFields([]metav1.ManagedFieldsEntry{statusOwner})
	client.PrependReactor("get", "*", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, before.DeepCopy(), nil
	})
	client.PrependReactor("patch", "*", func(action k8stesting.Action) (bool, runtime.Object, error) {
		patch, ok := action.(k8stesting.PatchAction)
		if !ok || patch.GetPatchType() != types.ApplyPatchType {
			t.Fatalf("status ownership must not generate a migration patch: %v", action)
		}
		return true, before.DeepCopy(), nil
	})
	result, err := ApplyResource(t.Context(), client, gvr, desired)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff([]metav1.ManagedFieldsEntry{statusOwner}, result.After.GetManagedFields()); diff != "" {
		t.Errorf("status ownership changed (-want +got):\n%s", diff)
	}
}

func TestApplyResource_RejectsMalformedLegacyOwnership(t *testing.T) {
	gvr := schema.GroupVersionResource{Group: "nais.io", Version: "v1alpha1", Resource: "applications"}
	client := dynfake.NewSimpleDynamicClient(runtime.NewScheme())
	desired := applyTestApplication()
	before := desired.DeepCopy()
	before.SetResourceVersion("42")
	before.SetManagedFields([]metav1.ManagedFieldsEntry{{
		Manager: "deployd", Operation: metav1.ManagedFieldsOperationUpdate,
		APIVersion: desired.GetAPIVersion(), FieldsType: "FieldsV1",
		FieldsV1: &metav1.FieldsV1{Raw: []byte(`{"f:spec":123}`)},
	}})
	client.PrependReactor("get", "*", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, before.DeepCopy(), nil
	})
	result, err := ApplyResource(t.Context(), client, gvr, desired)
	if err == nil || result != nil {
		t.Fatalf("expected invalid ownership to fail, got result=%+v, err=%v", result, err)
	}
	if actions := client.Actions(); len(actions) != 1 {
		t.Errorf("invalid ownership must not cause a write: %v", actions)
	}
}

func TestApplyResource_KubernetesErrors(t *testing.T) {
	gvr := schema.GroupVersionResource{Group: "nais.io", Version: "v1alpha1", Resource: "applications"}
	for _, test := range []struct {
		name     string
		verb     string
		existing bool
		err      error
	}{
		{name: "get forbidden", verb: "get", err: apierrors.NewForbidden(gvr.GroupResource(), "my-app", errors.New("denied"))},
		{name: "creation rejected", verb: "patch", err: apierrors.NewBadRequest("invalid resource")},
		{name: "existing apply rejected", verb: "patch", existing: true, err: apierrors.NewBadRequest("immutable field changed")},
		{name: "apply conflict", verb: "patch", existing: true, err: apierrors.NewConflict(gvr.GroupResource(), "my-app", errors.New("ownership conflict"))},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := dynfake.NewSimpleDynamicClient(runtime.NewScheme())
			desired := applyTestApplication()
			if test.existing {
				client.PrependReactor("get", "*", func(k8stesting.Action) (bool, runtime.Object, error) {
					return true, desired.DeepCopy(), nil
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

func applyTestApplication() *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "nais.io/v1alpha1",
		"kind":       "Application",
		"metadata":   map[string]any{"name": "my-app", "namespace": "my-team"},
		"spec": map[string]any{
			"image": "example.com/my-app:v1",
			"resources": map[string]any{
				"limits":   map[string]any{"cpu": "1", "memory": "512Mi"},
				"requests": map[string]any{"cpu": "100m", "memory": "256Mi"},
			},
		},
	}}
}

func applyAsManager(t *testing.T, client dynamic.ResourceInterface, object *unstructured.Unstructured, manager string) *unstructured.Unstructured {
	t.Helper()
	data, err := json.Marshal(object.Object)
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Patch(t.Context(), object.GetName(), types.ApplyPatchType, data, metav1.PatchOptions{FieldManager: manager})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func metadataWithoutBookkeeping(object *unstructured.Unstructured) any {
	copy := object.DeepCopy()
	copy.SetManagedFields(nil)
	copy.SetResourceVersion("")
	return copy.Object["metadata"]
}

func newFieldManagedDynamicClient(gvk schema.GroupVersionKind) *dynfake.FakeDynamicClient {
	scheme := runtime.NewScheme()
	scheme.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(gvk.GroupVersion().WithKind(gvk.Kind+"List"), &unstructured.UnstructuredList{})
	tracker := k8stesting.NewFieldManagedObjectTracker(scheme, serializer.NewCodecFactory(scheme).UniversalDecoder(), managedfields.NewDeducedTypeConverter())
	client := dynfake.NewSimpleDynamicClient(scheme)
	client.PrependReactor("*", "*", k8stesting.ObjectReaction(tracker))
	return client
}
