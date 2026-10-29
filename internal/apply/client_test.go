package apply

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
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

func TestApplyResource_Create(t *testing.T) {
	gvr := schema.GroupVersionResource{Group: "nais.io", Version: "v1alpha1", Resource: "applications"}
	client := dynfake.NewSimpleDynamicClient(runtime.NewScheme())
	desired := applyTestApplication()
	input := desired.DeepCopy()

	result, err := ApplyResource(t.Context(), client, gvr, desired)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Created || result.Before != nil {
		t.Fatal("expected creation without a before-state")
	}
	if diff := cmp.Diff(input.Object, result.After.Object); diff != "" {
		t.Errorf("created object mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(input.Object, desired.Object); diff != "" {
		t.Errorf("input was mutated (-want +got):\n%s", diff)
	}
	actions := client.Actions()
	if len(actions) != 2 || actions[0].GetVerb() != "get" || actions[1].GetVerb() != "create" {
		t.Fatalf("expected GET and CREATE, got %v", actions)
	}
	options := actions[1].(k8stesting.CreateActionImpl).GetCreateOptions()
	if options.FieldManager != fieldManager || options.FieldValidation != metav1.FieldValidationStrict {
		t.Errorf("unexpected create options: %+v", options)
	}
	stored, err := client.Resource(gvr).Namespace(desired.GetNamespace()).Get(t.Context(), desired.GetName(), metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(result.After.Object, stored.Object); diff != "" {
		t.Errorf("stored object mismatch (-want +got):\n%s", diff)
	}
}

func TestApplyResource_ReplacesRegardlessOfOwnership(t *testing.T) {
	for _, resource := range []struct {
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
		for _, owner := range []struct {
			manager string
			apply   bool
		}{
			{manager: fieldManager, apply: true},
			{manager: "kubectl-create"},
			{manager: "kubectl-client-side-apply"},
			{manager: "deployd"},
			{manager: "resource-controller", apply: true},
			{manager: "kubectl", apply: true},
		} {
			t.Run(resource.kind+"/"+owner.manager, func(t *testing.T) {
				gvk := resource.gvr.GroupVersion().WithKind(resource.kind)
				scheme := runtime.NewScheme()
				scheme.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})
				scheme.AddKnownTypeWithName(gvk.GroupVersion().WithKind(gvk.Kind+"List"), &unstructured.UnstructuredList{})
				tracker := k8stesting.NewFieldManagedObjectTracker(scheme, serializer.NewCodecFactory(scheme).UniversalDecoder(), managedfields.NewDeducedTypeConverter())
				client := dynfake.NewSimpleDynamicClient(scheme)
				client.PrependReactor("*", "*", k8stesting.ObjectReaction(tracker))
				resourceClient := client.Resource(resource.gvr).Namespace("my-team")
				existing := (&unstructured.Unstructured{Object: resource.beforeFields}).DeepCopy()
				existing.SetGroupVersionKind(gvk)
				existing.SetName("my-resource")
				existing.SetNamespace("my-team")
				existing.SetResourceVersion("42")
				existing.SetLabels(map[string]string{"kept": "same", "removed": "old"})
				existing.SetAnnotations(map[string]string{"removed": "old"})
				existing.SetFinalizers([]string{"controller.example.com/cleanup"})
				existing.SetOwnerReferences([]metav1.OwnerReference{{
					APIVersion: "nais.io/v1alpha1", Kind: "Application", Name: "owner", UID: "owner-uid",
				}})
				var before *unstructured.Unstructured
				var err error
				if owner.apply {
					data, marshalErr := json.Marshal(existing.Object)
					if marshalErr != nil {
						t.Fatal(marshalErr)
					}
					before, err = resourceClient.Patch(t.Context(), existing.GetName(), types.ApplyPatchType, data, metav1.PatchOptions{FieldManager: owner.manager})
				} else {
					before, err = resourceClient.Create(t.Context(), existing, metav1.CreateOptions{FieldManager: owner.manager})
				}
				if err != nil {
					t.Fatal(err)
				}
				desired := (&unstructured.Unstructured{Object: resource.desiredFields}).DeepCopy()
				desired.SetGroupVersionKind(gvk)
				desired.SetName(existing.GetName())
				desired.SetNamespace(existing.GetNamespace())
				desired.SetLabels(map[string]string{"kept": "same", "added": "new"})
				input := desired.DeepCopy()
				client.ClearActions()

				result, err := ApplyResource(t.Context(), client, resource.gvr, desired)
				if err != nil {
					t.Fatal(err)
				}
				if result.Created || result.Before == nil {
					t.Fatal("expected an existing resource")
				}
				if diff := cmp.Diff(before.Object, result.Before.Object); diff != "" {
					t.Errorf("before-state mismatch (-want +got):\n%s", diff)
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
					t.Fatalf("expected GET and UPDATE, got %v", actions)
				}
				update := actions[1].(k8stesting.UpdateActionImpl)
				options := update.GetUpdateOptions()
				if options.FieldManager != fieldManager || options.FieldValidation != metav1.FieldValidationStrict {
					t.Errorf("unexpected update options: %+v", options)
				}
				if version := update.GetObject().(*unstructured.Unstructured).GetResourceVersion(); version != "42" {
					t.Errorf("update resourceVersion = %q, want 42", version)
				}
				stored, err := resourceClient.Get(t.Context(), desired.GetName(), metav1.GetOptions{})
				if err != nil {
					t.Fatal(err)
				}
				if diff := cmp.Diff(result.After.Object, stored.Object); diff != "" {
					t.Errorf("stored object mismatch (-want +got):\n%s", diff)
				}
				reapplied, err := ApplyResource(t.Context(), client, resource.gvr, desired)
				if err != nil {
					t.Fatal(err)
				}
				if reapplied.Created || len(Diff(reapplied.Before, reapplied.After)) != 0 {
					t.Error("expected reapplication without configuration changes")
				}
			})
		}
	}
}

func TestApplyResource_ResourceVersion(t *testing.T) {
	gvr := schema.GroupVersionResource{Group: "nais.io", Version: "v1alpha1", Resource: "applications"}
	for _, test := range []struct {
		name             string
		submittedVersion string
		missing          bool
		concurrentUpdate bool
		wantConflict     bool
	}{
		{name: "uses fetched version when omitted"},
		{name: "accepts matching version", submittedVersion: "42"},
		{name: "rejects stale version", submittedVersion: "41", wantConflict: true},
		{name: "does not recreate deleted versioned object", submittedVersion: "42", missing: true, wantConflict: true},
		{name: "creates unversioned object", missing: true},
		{name: "rejects concurrent update without submitted version", concurrentUpdate: true, wantConflict: true},
		{name: "rejects concurrent update with submitted version", submittedVersion: "42", concurrentUpdate: true, wantConflict: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := dynfake.NewSimpleDynamicClient(runtime.NewScheme())
			desired := applyTestApplication()
			desired.SetResourceVersion(test.submittedVersion)
			input := desired.DeepCopy()
			before := applyTestApplication()
			before.SetResourceVersion("42")
			if !test.missing {
				client.PrependReactor("get", "*", func(k8stesting.Action) (bool, runtime.Object, error) {
					return true, before.DeepCopy(), nil
				})
			}
			client.PrependReactor("update", "*", func(action k8stesting.Action) (bool, runtime.Object, error) {
				submitted := action.(k8stesting.UpdateAction).GetObject().(*unstructured.Unstructured)
				if version := submitted.GetResourceVersion(); version != "42" {
					t.Errorf("update resourceVersion = %q, want 42", version)
				}
				if test.concurrentUpdate {
					return true, nil, apierrors.NewConflict(gvr.GroupResource(), desired.GetName(), errors.New("resourceVersion changed"))
				}
				after := submitted.DeepCopy()
				after.SetResourceVersion("43")
				return true, after, nil
			})

			result, err := ApplyResource(t.Context(), client, gvr, desired)
			if test.wantConflict {
				if !apierrors.IsConflict(err) || result != nil {
					t.Fatalf("expected concurrency conflict, got result=%+v, err=%v", result, err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if result.Created != test.missing {
					t.Errorf("created = %t, want %t", result.Created, test.missing)
				}
				if !test.missing {
					if diff := cmp.Diff(before.Object, result.Before.Object); diff != "" {
						t.Errorf("before-state mismatch (-want +got):\n%s", diff)
					}
				}
			}
			wantActions := 2
			if test.wantConflict && !test.concurrentUpdate {
				wantActions = 1
			}
			if actions := client.Actions(); len(actions) != wantActions {
				t.Errorf("expected %d actions, got %v", wantActions, actions)
			}
			if diff := cmp.Diff(input.Object, desired.Object); diff != "" {
				t.Errorf("input was mutated (-want +got):\n%s", diff)
			}
		})
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
		{name: "create forbidden", verb: "create", err: apierrors.NewForbidden(gvr.GroupResource(), "my-app", errors.New("denied"))},
		{name: "creation rejected", verb: "create", err: apierrors.NewBadRequest("invalid resource")},
		{name: "created concurrently", verb: "create", err: apierrors.NewAlreadyExists(gvr.GroupResource(), "my-app")},
		{name: "update forbidden", verb: "update", existing: true, err: apierrors.NewForbidden(gvr.GroupResource(), "my-app", errors.New("denied"))},
		{name: "update rejected", verb: "update", existing: true, err: apierrors.NewBadRequest("immutable field changed")},
		{name: "deleted concurrently", verb: "update", existing: true, err: apierrors.NewNotFound(gvr.GroupResource(), "my-app")},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := dynfake.NewSimpleDynamicClient(runtime.NewScheme())
			desired := applyTestApplication()
			if test.existing {
				before := desired.DeepCopy()
				before.SetResourceVersion("42")
				client.PrependReactor("get", "*", func(k8stesting.Action) (bool, runtime.Object, error) {
					return true, before.DeepCopy(), nil
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
