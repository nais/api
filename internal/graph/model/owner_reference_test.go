package model_test

import (
	"github.com/nais/api/internal/graph/model"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func TestOwnerReference(t *testing.T) {
	unsupportedRef := v1.OwnerReference{
		APIVersion: "v1",
		Kind:       "NotSupported",
		Name:       "job",
		UID:        "uid1",
	}

	t.Run("no refs", func(t *testing.T) {
		if o := model.OwnerReference([]v1.OwnerReference{}); o != nil {
			t.Fatalf("expected nil, got: %v", o)
		}
	})

	t.Run("with no supported refs", func(t *testing.T) {
		refs := []v1.OwnerReference{
			unsupportedRef,
		}

		if o := model.OwnerReference(refs); o != nil {
			t.Fatalf("expected nil, got: %v", o)
		}
	})

	t.Run("with application", func(t *testing.T) {
		expected := v1.OwnerReference{
			APIVersion: "v1",
			Kind:       "Application",
			Name:       "app",
			UID:        "uid2",
		}
		refs := []v1.OwnerReference{
			unsupportedRef,
			expected,
		}

		owner := model.OwnerReference(refs)
		if owner == nil {
			t.Fatalf("expected owner reference, got nil")
		}

		if *owner != expected {
			t.Fatalf("expected %v, got: %v", expected, *owner)
		}
	})

	t.Run("with NAIS job", func(t *testing.T) {
		expected := v1.OwnerReference{
			APIVersion: "v1",
			Kind:       "Naisjob",
			Name:       "job",
			UID:        "uid2",
		}
		refs := []v1.OwnerReference{
			unsupportedRef,
			expected,
		}

		owner := model.OwnerReference(refs)
		if owner == nil {
			t.Fatalf("expected owner reference, got nil")
		}

		if *owner != expected {
			t.Fatalf("expected %v, got: %v", expected, *owner)
		}
	})
}
