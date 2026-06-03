package pagination

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestNewFacetableConnection(t *testing.T) {
	ptr := func(s string) *string { return &s }

	tests := map[string]struct {
		edges         []Edge[string]
		pageInfo      PageInfo
		allItems      []string
		filter        *string
		expectedNodes []string
	}{
		"stores values and accessors return them": {
			edges: []Edge[string]{
				{Node: "a", Cursor: Cursor{Offset: 0}},
				{Node: "b", Cursor: Cursor{Offset: 1}},
			},
			pageInfo: PageInfo{
				TotalCount:  5,
				HasNextPage: true,
			},
			allItems:      []string{"a", "b", "c", "d", "e"},
			filter:        ptr("active"),
			expectedNodes: []string{"a", "b"},
		},
		"Nodes returns correct nodes from embedded connection": {
			edges: []Edge[string]{
				{Node: "x", Cursor: Cursor{Offset: 0}},
				{Node: "y", Cursor: Cursor{Offset: 1}},
				{Node: "z", Cursor: Cursor{Offset: 2}},
			},
			pageInfo:      PageInfo{TotalCount: 3},
			allItems:      []string{"x", "y", "z"},
			filter:        nil,
			expectedNodes: []string{"x", "y", "z"},
		},
		"edges and page info accessible through embedded connection": {
			edges: []Edge[string]{
				{Node: "one", Cursor: Cursor{Offset: 5}},
			},
			pageInfo: PageInfo{
				TotalCount:      10,
				HasNextPage:     true,
				HasPreviousPage: true,
				StartCursor:     &Cursor{Offset: 5},
				EndCursor:       &Cursor{Offset: 5},
			},
			allItems:      []string{"one"},
			filter:        ptr("some-filter"),
			expectedNodes: []string{"one"},
		},
		"empty connection": {
			edges:         nil,
			pageInfo:      PageInfo{},
			allItems:      nil,
			filter:        nil,
			expectedNodes: []string{},
		},
		"allItems differs from paginated nodes": {
			edges: []Edge[string]{
				{Node: "c", Cursor: Cursor{Offset: 2}},
				{Node: "d", Cursor: Cursor{Offset: 3}},
			},
			pageInfo: PageInfo{
				TotalCount:      5,
				HasNextPage:     true,
				HasPreviousPage: true,
			},
			allItems:      []string{"a", "b", "c", "d", "e"},
			filter:        ptr("all"),
			expectedNodes: []string{"c", "d"},
		},
	}

	opts := cmp.Options{cmpopts.IgnoreUnexported(Cursor{})}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			conn := &Connection[string]{
				Edges:    tc.edges,
				PageInfo: tc.pageInfo,
			}

			fc := NewFacetableConnection(conn, tc.allItems, tc.filter)

			// GetAllItems
			if diff := cmp.Diff(tc.allItems, fc.GetAllItems()); diff != "" {
				t.Errorf("GetAllItems() mismatch (-want +got):\n%s", diff)
			}

			// GetFilter
			if diff := cmp.Diff(tc.filter, fc.GetFilter()); diff != "" {
				t.Errorf("GetFilter() mismatch (-want +got):\n%s", diff)
			}

			// Nodes from embedded connection
			if diff := cmp.Diff(tc.expectedNodes, fc.Nodes()); diff != "" {
				t.Errorf("Nodes() mismatch (-want +got):\n%s", diff)
			}

			// Edges accessible
			if diff := cmp.Diff(tc.edges, fc.Edges, opts); diff != "" {
				t.Errorf("Edges mismatch (-want +got):\n%s", diff)
			}

			// PageInfo accessible
			if diff := cmp.Diff(tc.pageInfo, fc.PageInfo, opts); diff != "" {
				t.Errorf("PageInfo mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
