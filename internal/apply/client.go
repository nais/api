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
	"k8s.io/client-go/dynamic"
)

const fieldManager = "nais-api"

// ApplyResult holds the before and after states of a server-side apply operation.
type ApplyResult struct {
	// Before is the state of the object before the apply. Nil if the object did not exist.
	Before *unstructured.Unstructured
	// After is the state of the object after the apply.
	After *unstructured.Unstructured
	// Created is true if the object was created (did not exist before).
	Created bool
}

// ApplyResource performs a Kubernetes server-side apply for a single resource.
// It fetches the current state (before), applies the resource, and returns both
// before and after states so the caller can diff them.
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

	// Step 1: Get the current state of the object (before-state).
	before, err := resourceClient.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("getting current state of %s/%s: %w", namespace, name, err)
		}
		before = nil
	}

	// Step 2: Marshal the object to JSON for the apply patch.
	data, err := json.Marshal(obj.Object)
	if err != nil {
		return nil, fmt.Errorf("marshaling resource to JSON: %w", err)
	}

	// Step 3: Server-side apply using PATCH with ApplyPatchType.
	after, err := resourceClient.Patch(
		ctx,
		name,
		types.ApplyPatchType,
		data,
		metav1.PatchOptions{
			FieldManager: fieldManager,
			Force:        new(true),
		},
	)
	if err != nil {
		return nil, fmt.Errorf("applying %s/%s: %w", namespace, name, err)
	}

	return &ApplyResult{
		Before:  before,
		After:   after,
		Created: before == nil,
	}, nil
}
