package v0s

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"

	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/graph/pagination"
	protoapi "github.com/nais/v0s/api/v0s/v1"
)

const maxPageSize = 200

type pageCursor struct {
	Version   int    `json:"version"`
	Selection string `json:"selection"`
	Token     string `json:"token"`
	// Offset tracks the display position; only Token controls the backend seek.
	Offset int `json:"offset"`
}

type page struct {
	size      int
	cursor    pageCursor
	direction protoapi.PageDirection
}

func selectionKey(operation, filter string) string {
	sum := sha256.Sum256([]byte(operation + ":" + filter))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func parsePage(first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, selection string) (*page, error) {
	if first != nil && last != nil {
		first = nil
	}
	switch {
	case first != nil && before != nil:
		return nil, apierror.Errorf("first and before cannot be used together")
	case last != nil && after != nil:
		return nil, apierror.Errorf("last and after cannot be used together")
	case last != nil && before == nil:
		return nil, apierror.Errorf("last must be used with before")
	case after != nil && before != nil:
		return nil, apierror.Errorf("after and before cannot be used together")
	}
	p := &page{size: pagination.DefaultPageSize, cursor: pageCursor{Version: 1, Selection: selection}, direction: protoapi.PageDirection_PAGE_DIRECTION_FORWARD}
	boundary, name := after, "first"
	if before != nil {
		p.direction = protoapi.PageDirection_PAGE_DIRECTION_BACKWARD
		boundary, name = before, "last"
	}
	if first != nil {
		p.size = *first
	} else if last != nil {
		p.size = *last
	}
	if p.size < 1 || p.size > maxPageSize {
		return nil, apierror.Errorf("%s must be between 1 and %d", name, maxPageSize)
	}
	if boundary != nil {
		data, err := base64.RawURLEncoding.DecodeString(boundary.Keyset)
		var cursor pageCursor
		if err != nil || json.Unmarshal(data, &cursor) != nil ||
			cursor.Version != 1 || cursor.Selection != selection || cursor.Token == "" ||
			cursor.Offset < 0 || cursor.Offset > math.MaxInt32-maxPageSize {
			return nil, apierror.Errorf("Cursor does not belong to this vulnerability selection")
		}
		p.cursor = cursor
	}
	return p, nil
}

func readPage[T any](ctx context.Context, p *page,
	fetch func(context.Context, int32, string, protoapi.PageDirection) ([]T, []string, *protoapi.PageInfo, error), keep func(T) bool,
) ([]pagination.Edge[T], pagination.PageInfo, error) {
	edges := make([]pagination.Edge[T], 0, p.size+1)
	token := p.cursor.Token
	backward := p.direction == protoapi.PageDirection_PAGE_DIRECTION_BACKWARD
	info := pagination.PageInfo{}
	var initialInfo *protoapi.PageInfo
	seen := map[string]bool{}
	for {
		if err := ctx.Err(); err != nil {
			return nil, pagination.PageInfo{}, err
		}
		if seen[token] {
			return nil, pagination.PageInfo{}, fmt.Errorf("v0s returned a non-advancing cursor")
		}
		seen[token] = true
		values, cursors, backendInfo, err := fetchPage(ctx, fetch, int32(p.size+1-len(edges)), token, p.direction)
		if err != nil {
			return nil, pagination.PageInfo{}, err
		}
		if initialInfo == nil {
			initialInfo = backendInfo
		}
		chunk := make([]pagination.Edge[T], 0, len(values))
		for i, value := range values {
			if keep != nil && !keep(value) {
				continue
			}
			chunk = append(chunk, pagination.Edge[T]{Node: value, Cursor: pagination.Cursor{Keyset: cursors[i]}})
		}
		if backward {
			edges = append(chunk, edges...)
		} else {
			edges = append(edges, chunk...)
		}
		more := backendInfo.GetHasNextPage()
		if backward {
			more = backendInfo.GetHasPreviousPage()
		}
		if len(edges) > p.size || !more {
			break
		}
		if len(cursors) == 0 {
			return nil, pagination.PageInfo{}, fmt.Errorf("v0s returned an empty page with continuation")
		}
		token = cursors[len(cursors)-1]
		if backward {
			token = cursors[0]
		}
	}
	if len(edges) > p.size {
		if backward {
			info.HasPreviousPage = true
			edges = edges[len(edges)-p.size:]
		} else {
			info.HasNextPage = true
			edges = edges[:p.size]
		}
	}
	opposite := protoapi.PageDirection_PAGE_DIRECTION_BACKWARD
	oppositeExists := initialInfo.GetHasPreviousPage()
	if backward {
		opposite = protoapi.PageDirection_PAGE_DIRECTION_FORWARD
		oppositeExists = initialInfo.GetHasNextPage()
	}
	if oppositeExists {
		boundary := p.cursor.Token
		if len(edges) > 0 {
			boundary = edges[0].Cursor.Keyset
			if backward {
				boundary = edges[len(edges)-1].Cursor.Keyset
			}
		}
		exists := true
		if keep != nil {
			var err error
			exists, err = retainedOutside(ctx, fetch, keep, boundary, opposite, len(edges) == 0)
			if err != nil {
				return nil, pagination.PageInfo{}, err
			}
		}
		if backward {
			info.HasNextPage = exists
		} else {
			info.HasPreviousPage = exists
		}
	}
	offset := 0
	if backward {
		offset = max(0, p.cursor.Offset-len(edges))
	} else if p.cursor.Token != "" {
		offset = p.cursor.Offset + 1
	}
	for i := range edges {
		position := offset + i
		end := pageCursor{Version: 1, Selection: p.cursor.Selection, Token: edges[i].Cursor.Keyset, Offset: position}
		data, err := json.Marshal(end)
		if err != nil {
			return nil, pagination.PageInfo{}, fmt.Errorf("encode vulnerability cursor: %w", err)
		}
		edges[i].Cursor = pagination.Cursor{Keyset: base64.RawURLEncoding.EncodeToString(data), Offset: position}
	}
	if len(edges) > 0 {
		info.StartCursor = &edges[0].Cursor
		info.EndCursor = &edges[len(edges)-1].Cursor
	}
	return edges, info, nil
}

func fetchPage[T any](ctx context.Context,
	fetch func(context.Context, int32, string, protoapi.PageDirection) ([]T, []string, *protoapi.PageInfo, error),
	size int32, token string, direction protoapi.PageDirection,
) ([]T, []string, *protoapi.PageInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, nil, err
	}
	values, cursors, info, err := fetch(ctx, size, token, direction)
	if err != nil {
		return nil, nil, nil, err
	}
	if info == nil || len(values) != len(cursors) {
		return nil, nil, nil, apierror.Errorf("Vulnerability pagination is not available yet.")
	}
	if len(values) > int(size) {
		return nil, nil, nil, fmt.Errorf("v0s returned more items than requested")
	}
	for _, cursor := range cursors {
		if cursor == "" {
			return nil, nil, nil, fmt.Errorf("v0s returned an empty item cursor")
		}
	}
	return values, cursors, info, nil
}

func retainedOutside[T any](ctx context.Context,
	fetch func(context.Context, int32, string, protoapi.PageDirection) ([]T, []string, *protoapi.PageInfo, error),
	keep func(T) bool, boundary string, direction protoapi.PageDirection, inclusive bool,
) (bool, error) {
	if inclusive {
		_, cursors, _, err := fetchPage(ctx, fetch, 1, boundary, direction)
		if err != nil {
			return false, err
		}
		adjacent := ""
		if len(cursors) > 0 {
			adjacent = cursors[0]
		}
		reverse := protoapi.PageDirection_PAGE_DIRECTION_FORWARD
		if direction == reverse {
			reverse = protoapi.PageDirection_PAGE_DIRECTION_BACKWARD
		}
		values, cursors, _, err := fetchPage(ctx, fetch, 1, adjacent, reverse)
		if err != nil {
			return false, err
		}
		if len(cursors) > 0 && cursors[0] == boundary && (keep == nil || keep(values[0])) {
			return true, nil
		}
	}
	seen := map[string]bool{}
	for token := boundary; ; {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		if seen[token] {
			return false, fmt.Errorf("v0s returned a non-advancing cursor")
		}
		seen[token] = true
		values, cursors, info, err := fetchPage(ctx, fetch, maxPageSize, token, direction)
		if err != nil {
			return false, err
		}
		for _, value := range values {
			if keep == nil || keep(value) {
				return true, nil
			}
		}
		more := info.GetHasNextPage()
		if direction == protoapi.PageDirection_PAGE_DIRECTION_BACKWARD {
			more = info.GetHasPreviousPage()
		}
		if !more {
			return false, nil
		}
		if len(cursors) == 0 {
			return false, fmt.Errorf("v0s returned an empty page with continuation")
		}
		token = cursors[len(cursors)-1]
		if direction == protoapi.PageDirection_PAGE_DIRECTION_BACKWARD {
			token = cursors[0]
		}
	}
}
