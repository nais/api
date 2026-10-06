package apply

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	dynfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestApplyResource_MigrationAPIVersions(t *testing.T) {
	for _, test := range []struct {
		name        string
		owners      []metav1.ManagedFieldsEntry
		wantError   bool
		wantMigrate bool
	}{
		{
			name: "existing SSA version differs from legacy version",
			owners: []metav1.ManagedFieldsEntry{
				migrationTestOwner(fieldManager, metav1.ManagedFieldsOperationApply, "nais.io/v1", ""),
				migrationTestOwner("deployd", metav1.ManagedFieldsOperationUpdate, "nais.io/v1alpha1", ""),
			},
			wantError: true,
		},
		{
			name: "one legacy manager owns fields in multiple API versions",
			owners: []metav1.ManagedFieldsEntry{
				migrationTestOwner("deployd", metav1.ManagedFieldsOperationUpdate, "nais.io/v1", ""),
				migrationTestOwner("deployd", metav1.ManagedFieldsOperationUpdate, "nais.io/v1alpha1", ""),
			},
			wantError: true,
		},
		{
			name: "different legacy managers own different API versions",
			owners: []metav1.ManagedFieldsEntry{
				migrationTestOwner("kubectl-create", metav1.ManagedFieldsOperationUpdate, "nais.io/v1", ""),
				migrationTestOwner("deployd", metav1.ManagedFieldsOperationUpdate, "nais.io/v1alpha1", ""),
			},
			wantError: true,
		},
		{
			name: "matching SSA and legacy versions are migrated",
			owners: []metav1.ManagedFieldsEntry{
				migrationTestOwner(fieldManager, metav1.ManagedFieldsOperationApply, "nais.io/v1", ""),
				migrationTestOwner("deployd", metav1.ManagedFieldsOperationUpdate, "nais.io/v1", ""),
			},
			wantMigrate: true,
		},
		{
			name: "a single older version can be migrated before SSA version conversion",
			owners: []metav1.ManagedFieldsEntry{
				migrationTestOwner("deployd", metav1.ManagedFieldsOperationUpdate, "nais.io/v1alpha1", ""),
			},
			wantMigrate: true,
		},
		{
			name: "foreign manager and status versions do not prevent migration",
			owners: []metav1.ManagedFieldsEntry{
				migrationTestOwner(fieldManager, metav1.ManagedFieldsOperationApply, "nais.io/v1", ""),
				migrationTestOwner("deployd", metav1.ManagedFieldsOperationUpdate, "nais.io/v1", ""),
				migrationTestOwner("deployd", metav1.ManagedFieldsOperationUpdate, "nais.io/v1alpha1", "status"),
				migrationTestOwner("controller", metav1.ManagedFieldsOperationUpdate, "nais.io/v1alpha1", ""),
			},
			wantMigrate: true,
		},
		{
			name: "legacy Apply entries are not migrated regardless of their version",
			owners: []metav1.ManagedFieldsEntry{
				migrationTestOwner(fieldManager, metav1.ManagedFieldsOperationApply, "nais.io/v1", ""),
				migrationTestOwner("deployd", metav1.ManagedFieldsOperationApply, "nais.io/v1alpha1", ""),
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			gvr := schema.GroupVersionResource{Group: "nais.io", Version: "v1", Resource: "naisjobs"}
			client := dynfake.NewSimpleDynamicClient(runtime.NewScheme())
			desired := applyTestApplication()
			desired.SetAPIVersion("nais.io/v1")
			desired.SetKind("Naisjob")
			before := desired.DeepCopy()
			before.SetResourceVersion("42")
			before.SetManagedFields(test.owners)
			snapshot := before.DeepCopy()
			migrations := 0
			applies := 0
			client.PrependReactor("get", "*", func(k8stesting.Action) (bool, runtime.Object, error) {
				return true, before.DeepCopy(), nil
			})
			client.PrependReactor("patch", "*", func(action k8stesting.Action) (bool, runtime.Object, error) {
				patch, ok := action.(k8stesting.PatchAction)
				if !ok {
					t.Fatalf("expected PatchAction, got %T", action)
				}
				if patch.GetPatchType() == types.JSONPatchType {
					migrations++
					migrated := before.DeepCopy()
					migrated.SetResourceVersion("43")
					return true, migrated, nil
				}
				if patch.GetPatchType() != types.ApplyPatchType {
					t.Fatalf("unexpected patch type: %s", patch.GetPatchType())
				}
				applies++
				return true, desired.DeepCopy(), nil
			})

			result, err := ApplyResource(t.Context(), client, gvr, desired)
			if test.wantError {
				if err == nil || result != nil {
					t.Fatalf("expected unsupported migration to fail, got result=%+v, err=%v", result, err)
				}
				if actions := client.Actions(); len(actions) != 1 || migrations != 0 || applies != 0 {
					t.Errorf("unsupported migration performed a write: %v", actions)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				wantMigrations := 0
				if test.wantMigrate {
					wantMigrations = 1
				}
				if migrations != wantMigrations || applies != 1 {
					t.Errorf("migration/apply counts = %d/%d, want %d/1", migrations, applies, wantMigrations)
				}
			}
			if diff := cmp.Diff(snapshot.Object, before.Object); diff != "" {
				t.Errorf("migration mutated the before-state (-want +got):\n%s", diff)
			}
		})
	}
}

func TestApplyResource_CallerResourceVersion(t *testing.T) {
	for _, test := range []struct {
		name              string
		submittedVersion  string
		migrate           bool
		missing           bool
		concurrentUpdate  bool
		wantConflict      bool
		wantApplyVersion  string
		wantMigrationCall bool
	}{
		{
			name: "current version is advanced after migration", submittedVersion: "42", migrate: true,
			wantApplyVersion: "43", wantMigrationCall: true,
		},
		{
			name: "current version is retained without migration", submittedVersion: "42",
			wantApplyVersion: "42",
		},
		{
			name: "absent version stays absent after migration", migrate: true, wantMigrationCall: true,
		},
		{
			name: "stale version cannot migrate ownership", submittedVersion: "41", migrate: true, wantConflict: true,
		},
		{
			name: "stale version cannot apply without migration", submittedVersion: "41", wantConflict: true,
		},
		{
			name: "a deleted versioned object is not recreated", submittedVersion: "42", missing: true, wantConflict: true,
		},
		{
			name: "unversioned creation remains supported", missing: true,
		},
		{
			name: "concurrent update after migration is still rejected", submittedVersion: "42", migrate: true,
			concurrentUpdate: true, wantConflict: true, wantApplyVersion: "43", wantMigrationCall: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			gvr := schema.GroupVersionResource{Group: "nais.io", Version: "v1alpha1", Resource: "applications"}
			client := dynfake.NewSimpleDynamicClient(runtime.NewScheme())
			desired := applyTestApplication()
			desired.SetResourceVersion(test.submittedVersion)
			input := desired.DeepCopy()
			before := applyTestApplication()
			before.SetResourceVersion("42")
			if test.migrate {
				before.SetManagedFields([]metav1.ManagedFieldsEntry{
					migrationTestOwner("deployd", metav1.ManagedFieldsOperationUpdate, before.GetAPIVersion(), ""),
				})
			}
			currentVersion := "42"
			migrations := 0
			applies := 0
			client.PrependReactor("get", "*", func(k8stesting.Action) (bool, runtime.Object, error) {
				if test.missing {
					return true, nil, apierrors.NewNotFound(gvr.GroupResource(), desired.GetName())
				}
				return true, before.DeepCopy(), nil
			})
			client.PrependReactor("patch", "*", func(action k8stesting.Action) (bool, runtime.Object, error) {
				patch, ok := action.(k8stesting.PatchAction)
				if !ok {
					t.Fatalf("expected PatchAction, got %T", action)
				}
				if patch.GetPatchType() == types.JSONPatchType {
					migrations++
					currentVersion = "43"
					migrated := before.DeepCopy()
					migrated.SetResourceVersion(currentVersion)
					if test.concurrentUpdate {
						currentVersion = "44"
					}
					return true, migrated, nil
				}
				if patch.GetPatchType() != types.ApplyPatchType {
					t.Fatalf("unexpected patch type: %s", patch.GetPatchType())
				}
				applies++
				var submitted unstructured.Unstructured
				if err := json.Unmarshal(patch.GetPatch(), &submitted); err != nil {
					t.Fatal(err)
				}
				if submitted.GetResourceVersion() != test.wantApplyVersion {
					t.Errorf("SSA resourceVersion = %q, want %q", submitted.GetResourceVersion(), test.wantApplyVersion)
				}
				if version := submitted.GetResourceVersion(); version != "" && version != currentVersion {
					return true, nil, apierrors.NewConflict(gvr.GroupResource(), desired.GetName(), nil)
				}
				submitted.SetResourceVersion("45")
				return true, &submitted, nil
			})

			result, err := ApplyResource(t.Context(), client, gvr, desired)
			if test.wantConflict {
				if !apierrors.IsConflict(err) || result != nil {
					t.Fatalf("expected concurrency conflict, got result=%+v, err=%v", result, err)
				}
				if !test.concurrentUpdate && applies != 0 {
					t.Error("stale request reached SSA")
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if result.Created != test.missing || applies != 1 {
					t.Errorf("unexpected create/apply result: created=%t, applies=%d", result.Created, applies)
				}
				if !test.missing {
					if diff := cmp.Diff(before.Object, result.Before.Object); diff != "" {
						t.Errorf("before-state mismatch (-want +got):\n%s", diff)
					}
				}
			}
			wantMigrations := 0
			if test.wantMigrationCall {
				wantMigrations = 1
			}
			if migrations != wantMigrations {
				t.Errorf("migration calls = %d, want %d", migrations, wantMigrations)
			}
			if diff := cmp.Diff(input.Object, desired.Object); diff != "" {
				t.Errorf("caller input was mutated (-want +got):\n%s", diff)
			}
		})
	}
}

func migrationTestOwner(manager string, operation metav1.ManagedFieldsOperationType, apiVersion, subresource string) metav1.ManagedFieldsEntry {
	return metav1.ManagedFieldsEntry{
		Manager: manager, Operation: operation, APIVersion: apiVersion, Subresource: subresource, FieldsType: "FieldsV1",
		FieldsV1: &metav1.FieldsV1{Raw: []byte(`{"f:spec":{"f:resources":{"f:limits":{"f:cpu":{}}}}}`)},
	}
}
