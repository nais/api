package apply

import (
	"context"
	"encoding/json"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/util/csaupgrade"
)

const fieldManager = "nais-api"

// ApplyResult holds the before and after states of an apply operation.
type ApplyResult struct {
	// Before is the state of the object before the apply. Nil if the object did not exist.
	Before *unstructured.Unstructured
	// After is the state of the object after the apply.
	After *unstructured.Unstructured
	// Created is true if the object was created (did not exist before).
	Created bool
}

// ApplyResource uses server-side apply after migrating ownership from legacy deployers.
// Other field managers retain their ownership, and conflicting changes are rejected.
// It returns both before and after states so the caller can diff them.
func ApplyResource(
	ctx context.Context,
	client dynamic.Interface,
	gvr schema.GroupVersionResource,
	obj *unstructured.Unstructured,
) (*ApplyResult, error) {
	namespace := obj.GetNamespace()
	name := obj.GetName()

	if name == "" {
		return nil, fmt.Errorf("resource must have a name")
	}
	if namespace == "" {
		return nil, fmt.Errorf("resource must have a namespace")
	}

	resourceClient := client.Resource(gvr).Namespace(namespace)

	before, err := resourceClient.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("getting current state of %s/%s: %w", namespace, name, err)
		}
		before = nil
	}

	submittedVersion := obj.GetResourceVersion()
	if submittedVersion != "" {
		if before == nil {
			return nil, apierrors.NewConflict(gvr.GroupResource(), name, fmt.Errorf("resourceVersion %q refers to a resource that no longer exists", submittedVersion))
		}
		if submittedVersion != before.GetResourceVersion() {
			return nil, apierrors.NewConflict(gvr.GroupResource(), name, fmt.Errorf("requested resourceVersion %q does not match current version %q", submittedVersion, before.GetResourceVersion()))
		}
	}
	desired := obj.DeepCopy()
	if before != nil {
		legacyManagers := sets.New("kubectl-create", "kubectl-client-side-apply", "deployd")
		if err := validateMigrationVersions(before.GetManagedFields(), legacyManagers); err != nil {
			return nil, fmt.Errorf("preparing ownership migration for %s/%s: %w", namespace, name, err)
		}
		// Transfer legacy Update ownership to SSA so omitted fields can be removed
		// without taking ownership from other managers.
		patch, err := csaupgrade.UpgradeManagedFieldsPatch(
			before,
			legacyManagers,
			fieldManager,
		)
		if err != nil {
			return nil, fmt.Errorf("preparing ownership migration for %s/%s: %w", namespace, name, err)
		}
		if patch != nil {
			migrated, err := resourceClient.Patch(ctx, name, types.JSONPatchType, patch, metav1.PatchOptions{
				FieldManager: fieldManager,
			})
			if err != nil {
				return nil, fmt.Errorf("migrating field ownership for %s/%s: %w", namespace, name, err)
			}
			if submittedVersion != "" {
				if migrated == nil || migrated.GetResourceVersion() == "" {
					return nil, fmt.Errorf("ownership migration for %s/%s returned no resourceVersion", namespace, name)
				}
				desired.SetResourceVersion(migrated.GetResourceVersion())
			}
		}
	}

	data, err := json.Marshal(desired.Object)
	if err != nil {
		return nil, fmt.Errorf("marshaling resource to JSON: %w", err)
	}
	after, err := resourceClient.Patch(ctx, name, types.ApplyPatchType, data, metav1.PatchOptions{
		FieldManager:    fieldManager,
		Force:           new(false),
		FieldValidation: metav1.FieldValidationStrict,
	})
	if err != nil {
		return nil, fmt.Errorf("applying %s/%s: %w", namespace, name, err)
	}

	return &ApplyResult{
		Before:  before,
		After:   after,
		Created: before == nil,
	}, nil
}

func validateMigrationVersions(entries []metav1.ManagedFieldsEntry, legacyManagers sets.Set[string]) error {
	var targetVersion string
	for _, entry := range entries {
		if entry.Manager == fieldManager && entry.Operation == metav1.ManagedFieldsOperationApply && entry.Subresource == "" {
			targetVersion = entry.APIVersion
			break
		}
	}
	// csaupgrade drops legacy entries in other API versions without merging them.
	for _, entry := range entries {
		if !legacyManagers.Has(entry.Manager) || entry.Operation != metav1.ManagedFieldsOperationUpdate || entry.Subresource != "" {
			continue
		}
		if targetVersion == "" {
			targetVersion = entry.APIVersion
		}
		if entry.APIVersion != targetVersion {
			return fmt.Errorf("cannot safely migrate %q ownership from API version %q into %q ownership for API version %q", entry.Manager, entry.APIVersion, fieldManager, targetVersion)
		}
	}
	return nil
}
