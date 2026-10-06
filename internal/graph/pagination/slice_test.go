package pagination_test

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nais/api/internal/graph/pagination"
)

func TestSlice(t *testing.T) {
	t.Run("empty slice", func(t *testing.T) {
		page, _ := pagination.ParsePage(new(10), nil, nil, nil)

		if got := pagination.Slice([]string{}, page); len(got) != 0 {
			t.Errorf("Expected empty slice, got: %v", got)
		}
	})

	t.Run("non empty slice", func(t *testing.T) {
		page, _ := pagination.ParsePage(new(2), nil, nil, nil)
		expected := []int{1, 2}
		got := pagination.Slice([]int{1, 2, 3, 4}, page)

		if diff := cmp.Diff(expected, got); diff != "" {
			t.Errorf("diff: -want +got\n%s", diff)
		}
	})

	t.Run("slice smaller than offset", func(t *testing.T) {
		page, _ := pagination.ParsePage(new(2), &pagination.Cursor{Offset: 5}, nil, nil)

		if got := pagination.Slice([]int{1, 2}, page); len(got) != 0 {
			t.Errorf("Expected empty slice, got: %v", got)
		}
	})

	t.Run("offset", func(t *testing.T) {
		page, _ := pagination.ParsePage(new(3), &pagination.Cursor{Offset: 5}, nil, nil)
		expected := []int{7, 8, 9}
		got := pagination.Slice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, page)

		if diff := cmp.Diff(expected, got); diff != "" {
			t.Errorf("diff: -want +got\n%s", diff)
		}
	})
}

func TestSliceBeforeBoundary(t *testing.T) {
	var nodes []int
	for index := 0; index < 25; index++ {
		nodes = append(nodes, index)
	}
	for _, test := range []struct {
		before int
		start  int
		end    int
	}{
		{before: 0, start: 0, end: 0},
		{before: 1, start: 0, end: 1},
		{before: 5, start: 0, end: 5},
		{before: 20, start: 0, end: 20},
		{before: 25, start: 5, end: 25},
	} {
		t.Run(fmt.Sprintf("before_%d", test.before), func(t *testing.T) {
			page, err := pagination.ParsePage(nil, nil, new(20), &pagination.Cursor{Offset: test.before})
			if err != nil {
				t.Fatal(err)
			}
			connection := pagination.NewConnection(pagination.Slice(nodes, page), page, len(nodes))
			if diff := cmp.Diff(nodes[test.start:test.end], connection.Nodes()); diff != "" {
				t.Fatalf("diff: -want +got\n%s", diff)
			}
			if connection.PageInfo.TotalCount != len(nodes) || connection.PageInfo.HasNextPage != (test.end < len(nodes)) || connection.PageInfo.HasPreviousPage != (test.start > 0) {
				t.Fatalf("incorrect pageInfo: %+v", connection.PageInfo)
			}
			if test.start == test.end {
				if connection.PageInfo.StartCursor != nil || connection.PageInfo.EndCursor != nil {
					t.Fatal("empty page must have null cursors")
				}
			} else if connection.PageInfo.StartCursor.Offset != test.start || connection.PageInfo.EndCursor.Offset != test.end-1 {
				t.Fatalf("incorrect cursors: %+v", connection.PageInfo)
			}
		})
	}
}
