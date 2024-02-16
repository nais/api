package model_test

import (
	"testing"

	"github.com/nais/api/internal/graph/model"
	"k8s.io/utils/ptr"
)

func Test_NewPagination(t *testing.T) {
	t.Run("no pagination", func(t *testing.T) {
		pagination := model.NewPagination(nil, nil)
		if pagination.Offset != 0 {
			t.Errorf("expected 0, got %d", pagination.Offset)
		}

		if pagination.Limit != 20 {
			t.Errorf("expected 20, got %d", pagination.Limit)
		}
	})

	t.Run("pagination with values", func(t *testing.T) {
		pagination := model.NewPagination(ptr.To(42), ptr.To(1337))
		if pagination.Offset != 42 {
			t.Errorf("expected 42, got %d", pagination.Offset)
		}

		if pagination.Limit != 1337 {
			t.Errorf("expected 1337, got %d", pagination.Limit)
		}
	})
}
