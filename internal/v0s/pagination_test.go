package v0s

import (
	"context"
	"errors"
	"slices"
	"sort"
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nais/api/internal/graph/pagination"
	protoapi "github.com/nais/v0s/api/v0s/v1"
)

func integerPages(items []int) func(context.Context, int32, string, protoapi.PageDirection) ([]int, []string, *protoapi.PageInfo, error) {
	return func(_ context.Context, size int32, token string, direction protoapi.PageDirection) ([]int, []string, *protoapi.PageInfo, error) {
		start, end := 0, len(items)
		if token != "" {
			boundary, err := strconv.Atoi(token)
			if err != nil {
				return nil, nil, nil, err
			}
			if direction == protoapi.PageDirection_PAGE_DIRECTION_BACKWARD {
				end = sort.SearchInts(items, boundary)
			} else {
				start = sort.Search(len(items), func(i int) bool { return items[i] > boundary })
			}
		}
		if direction == protoapi.PageDirection_PAGE_DIRECTION_BACKWARD {
			start = max(0, end-int(size))
		} else {
			end = min(len(items), start+int(size))
		}
		cursors := make([]string, end-start)
		for i, value := range items[start:end] {
			cursors[i] = strconv.Itoa(value)
		}
		return items[start:end], cursors, &protoapi.PageInfo{
			HasPreviousPage: start > 0, HasNextPage: end < len(items),
		}, nil
	}
}

func TestReadPageRoundTripWithFilteredChunks(t *testing.T) {
	fetch := integerPages([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10})
	keep := func(value int) bool { return value%2 == 0 }
	read := func(first *int, after *pagination.Cursor, last *int, before *pagination.Cursor) ([]pagination.Edge[int], pagination.PageInfo) {
		t.Helper()
		p, err := parsePage(first, after, last, before, "selection")
		if err != nil {
			t.Fatal(err)
		}
		edges, info, err := readPage(t.Context(), p, fetch, keep)
		if err != nil {
			t.Fatal(err)
		}
		return edges, info
	}
	first, firstInfo := read(new(2), nil, nil, nil)
	next, nextInfo := read(new(2), firstInfo.EndCursor, nil, nil)
	back, backInfo := read(nil, nil, new(2), nextInfo.StartCursor)
	for _, tt := range []struct {
		edges []pagination.Edge[int]
		info  pagination.PageInfo
		want  []int
		prev  bool
		next  bool
		start int
		end   int
	}{
		{first, firstInfo, []int{2, 4}, false, true, 1, 2},
		{next, nextInfo, []int{6, 8}, true, true, 3, 4},
		{back, backInfo, []int{2, 4}, false, true, 1, 2},
	} {
		got := make([]int, len(tt.edges))
		for i, edge := range tt.edges {
			got[i] = edge.Node
		}
		if diff := cmp.Diff(tt.want, got); diff != "" {
			t.Errorf("page nodes (-want +got):\n%s", diff)
		}
		if tt.info.HasPreviousPage != tt.prev || tt.info.HasNextPage != tt.next ||
			tt.info.PageStart() != tt.start || tt.info.PageEnd() != tt.end {
			t.Errorf("page info: %+v", tt.info)
		}
	}
	if !slices.Equal(first, back) {
		t.Errorf("roundtrip changed canonical cursors: first=%v, back=%v", first, back)
	}
	before, beforeInfo := read(nil, nil, new(20), nextInfo.StartCursor)
	if len(before) != 2 || beforeInfo.HasPreviousPage || !beforeInfo.HasNextPage {
		t.Fatalf("short backward page crossed its exclusive boundary: %v, %+v", before, beforeInfo)
	}
}

func TestReadPageEmptyAndDeletedBoundaries(t *testing.T) {
	for _, tt := range []struct {
		name      string
		items     []int
		token     string
		direction protoapi.PageDirection
		keep      func(int) bool
		prev      bool
		next      bool
	}{
		{"after retained last", []int{1, 2}, "2", protoapi.PageDirection_PAGE_DIRECTION_FORWARD, func(v int) bool { return v == 2 }, true, false},
		{"before retained first", []int{1, 2}, "1", protoapi.PageDirection_PAGE_DIRECTION_BACKWARD, func(v int) bool { return v == 1 }, false, true},
		{"after deleted last", []int{1}, "2", protoapi.PageDirection_PAGE_DIRECTION_FORWARD, func(int) bool { return true }, true, false},
		{"before deleted first", []int{2}, "1", protoapi.PageDirection_PAGE_DIRECTION_BACKWARD, func(int) bool { return true }, false, true},
		{"only rejected rows after", []int{1, 2, 3}, "1", protoapi.PageDirection_PAGE_DIRECTION_FORWARD, func(int) bool { return false }, false, false},
		{"only rejected rows before", []int{1, 2, 3}, "3", protoapi.PageDirection_PAGE_DIRECTION_BACKWARD, func(int) bool { return false }, false, false},
		{"empty selection", nil, "2", protoapi.PageDirection_PAGE_DIRECTION_BACKWARD, func(int) bool { return true }, false, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p := &page{size: 2, cursor: pageCursor{Version: 1, Selection: "selection", Token: tt.token, Offset: 1}, direction: tt.direction}
			edges, info, err := readPage(t.Context(), p, integerPages(tt.items), tt.keep)
			if err != nil {
				t.Fatal(err)
			}
			if len(edges) != 0 || info.StartCursor != nil || info.EndCursor != nil ||
				info.HasPreviousPage != tt.prev || info.HasNextPage != tt.next || info.PageStart() != 0 || info.PageEnd() != 0 {
				t.Fatalf("empty page: %v, %+v", edges, info)
			}
		})
	}
}

func TestReadPageProbesDiscardedRowsOutsideBothEdges(t *testing.T) {
	items := make([]int, 450)
	for i := range items {
		items[i] = i + 1
	}
	fetch := integerPages(items)
	keep := func(v int) bool { return slices.Contains([]int{1, 225, 450}, v) }
	for _, direction := range []protoapi.PageDirection{
		protoapi.PageDirection_PAGE_DIRECTION_FORWARD, protoapi.PageDirection_PAGE_DIRECTION_BACKWARD,
	} {
		token := "224"
		if direction == protoapi.PageDirection_PAGE_DIRECTION_BACKWARD {
			token = "226"
		}
		p := &page{size: 1, cursor: pageCursor{Selection: "selection", Token: token}, direction: direction}
		edges, info, err := readPage(t.Context(), p, fetch, keep)
		if err != nil {
			t.Fatal(err)
		}
		if len(edges) != 1 || edges[0].Node != 225 || !info.HasNextPage || !info.HasPreviousPage {
			t.Fatalf("lost a retained row beyond a rejected chunk: %v, %+v", edges, info)
		}
	}
}

func TestReadPageRejectsUnavailableAndNonAdvancingPagination(t *testing.T) {
	wantErr := errors.New("backend failed")
	for _, tt := range []struct {
		name  string
		fetch func(context.Context, int32, string, protoapi.PageDirection) ([]int, []string, *protoapi.PageInfo, error)
	}{
		{"missing metadata", func(context.Context, int32, string, protoapi.PageDirection) ([]int, []string, *protoapi.PageInfo, error) {
			return []int{1}, []string{"1"}, nil, nil
		}},
		{"missing cursor", func(context.Context, int32, string, protoapi.PageDirection) ([]int, []string, *protoapi.PageInfo, error) {
			return []int{1}, nil, &protoapi.PageInfo{}, nil
		}},
		{"empty cursor", func(context.Context, int32, string, protoapi.PageDirection) ([]int, []string, *protoapi.PageInfo, error) {
			return []int{1}, []string{""}, &protoapi.PageInfo{}, nil
		}},
		{"repeated cursor", func(context.Context, int32, string, protoapi.PageDirection) ([]int, []string, *protoapi.PageInfo, error) {
			return []int{1}, []string{"same"}, &protoapi.PageInfo{HasNextPage: true, HasPreviousPage: true}, nil
		}},
		{"backend error", func(context.Context, int32, string, protoapi.PageDirection) ([]int, []string, *protoapi.PageInfo, error) {
			return nil, nil, nil, wantErr
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			for _, direction := range []protoapi.PageDirection{protoapi.PageDirection_PAGE_DIRECTION_FORWARD, protoapi.PageDirection_PAGE_DIRECTION_BACKWARD} {
				p := &page{size: 2, direction: direction}
				_, _, err := readPage(t.Context(), p, tt.fetch, func(int) bool { return false })
				if err == nil {
					t.Fatal("invalid pagination was accepted")
				}
				if tt.name == "backend error" && !errors.Is(err, wantErr) {
					t.Fatalf("backend error lost: %v", err)
				}
			}
		})
	}
}

func TestParsePageMatchesBidirectionalArgumentRules(t *testing.T) {
	edges, _, err := readPage(t.Context(), &page{size: 2, cursor: pageCursor{Selection: "selection"}, direction: protoapi.PageDirection_PAGE_DIRECTION_FORWARD}, integerPages([]int{1, 2}), nil)
	if err != nil {
		t.Fatal(err)
	}
	cursor := &edges[1].Cursor
	for _, tt := range []struct {
		name          string
		first, last   *int
		after, before *pagination.Cursor
		selection     string
		wantErr       bool
		size          int
		direction     protoapi.PageDirection
	}{
		{"last wins", new(1), new(2), nil, cursor, "selection", false, 2, protoapi.PageDirection_PAGE_DIRECTION_BACKWARD},
		{"before defaults", nil, nil, nil, cursor, "selection", false, 20, protoapi.PageDirection_PAGE_DIRECTION_BACKWARD},
		{"first before", new(1), nil, nil, cursor, "selection", true, 0, 0},
		{"last after", nil, new(1), cursor, nil, "selection", true, 0, 0},
		{"last alone", nil, new(1), nil, nil, "selection", true, 0, 0},
		{"both cursors", nil, nil, cursor, cursor, "selection", true, 0, 0},
		{"zero last", nil, new(0), nil, cursor, "selection", true, 0, 0},
		{"large last", nil, new(201), nil, cursor, "selection", true, 0, 0},
		{"changed selection", nil, new(1), nil, cursor, "changed", true, 0, 0},
		{"offset cursor", nil, new(1), nil, &pagination.Cursor{Offset: 1}, "selection", true, 0, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p, err := parsePage(tt.first, tt.after, tt.last, tt.before, tt.selection)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parse page error: %v", err)
			}
			if err == nil && (p.size != tt.size || p.direction != tt.direction) {
				t.Fatalf("page: %+v", p)
			}
		})
	}
}
