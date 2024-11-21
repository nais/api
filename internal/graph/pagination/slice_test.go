package pagination_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nais/api/internal/graph/pagination"
	"k8s.io/utils/ptr"
)

func TestSlice(t *testing.T) {
	t.Run("empty slice", func(t *testing.T) {
		page, _ := pagination.ParsePage(ptr.To(10), nil, nil, nil)

		if got := pagination.Slice([]string{}, page); len(got) != 0 {
			t.Errorf("Expected empty slice, got: %v", got)
		}
	})

	t.Run("non empty slice", func(t *testing.T) {
		page, _ := pagination.ParsePage(ptr.To(2), nil, nil, nil)
		expected := []int{1, 2}
		got := pagination.Slice([]int{1, 2, 3, 4}, page)

		if diff := cmp.Diff(expected, got); diff != "" {
			t.Errorf("diff: -want +got\n%s", diff)
		}
	})

	t.Run("slice smaller than offset", func(t *testing.T) {
		page, _ := pagination.ParsePage(ptr.To(2), &pagination.Cursor{Offset: 5}, nil, nil)

		if got := pagination.Slice([]int{1, 2}, page); len(got) != 0 {
			t.Errorf("Expected empty slice, got: %v", got)
		}
	})

	t.Run("offset", func(t *testing.T) {
		page, _ := pagination.ParsePage(ptr.To(3), &pagination.Cursor{Offset: 5}, nil, nil)
		expected := []int{7, 8, 9}
		got := pagination.Slice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, page)

		if diff := cmp.Diff(expected, got); diff != "" {
			t.Errorf("diff: -want +got\n%s", diff)
		}
	})
}
