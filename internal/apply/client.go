package apply

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
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

// ApplyResource creates or replaces a Kubernetes resource with the submitted object.
// Existing resources are updated using their current resourceVersion to detect conflicts.
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

	desired := obj.DeepCopy()
	var after *unstructured.Unstructured
	if before == nil {
		after, err = resourceClient.Create(ctx, desired, metav1.CreateOptions{
			FieldManager:    fieldManager,
			FieldValidation: metav1.FieldValidationStrict,
		})
	} else {
		desired.SetResourceVersion(before.GetResourceVersion())
		after, err = resourceClient.Update(ctx, desired, metav1.UpdateOptions{
			FieldManager:    fieldManager,
			FieldValidation: metav1.FieldValidationStrict,
		})
	}
	if err != nil {
		return nil, fmt.Errorf("applying %s/%s: %w", namespace, name, err)
	}

	return &ApplyResult{
		Before:  before,
		After:   after,
		Created: before == nil,
	}, nil
}
